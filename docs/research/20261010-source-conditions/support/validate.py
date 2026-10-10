import hashlib, json, pathlib, re

import sys
root = pathlib.Path(sys.argv[1]).resolve()
revision = '92dc06b38f72518443f8a90e5c98a645f0c756f1'
def read(name): return json.loads((root/name).read_text())
def sha(raw): return 'sha256:'+hashlib.sha256(raw).hexdigest()
def file_sha(name): return sha((root/name).read_bytes())
def walk(value):
    if isinstance(value,dict):
        yield value
        for child in value.values(): yield from walk(child)
    elif isinstance(value,list):
        for child in value: yield from walk(child)

oracle = read('oracle.json')
inputs = read('inputs.json')
by_input = {row['input']:row for row in oracle}
assert [row['input'] for row in oracle] == inputs and len(by_input)==268
assert set(range(256)) <= set(inputs)
assert {-9223372036854775808,-9007199254740993,9007199254740993,9223372036854775807} <= set(inputs)
assert ''.join(line for line in (root/'constrained.gooo').read_text().splitlines(True) if 'condition_case ' not in line) == (root/'baseline.gooo').read_text()
condition_cases = [(int(x), y=='true') for x,y in re.findall(r'condition_case "outside-branch" input "(-?\d+)" -> "(true|false)"',(root/'constrained.gooo').read_text())]
assert len(condition_cases)==13 and len({x for x,y in condition_cases})==13
assert all(expected is by_input[value]['outside'] for value,expected in condition_cases)

candidate = read('candidate-identity.stdout.json')
published = read('published-identity.stdout.json')
assert candidate['compiler_source_sha']==revision and candidate['source_status']=='CLEAN_VCS'
assert candidate['go_version']=='go1.27.2'
assert published['compiler_source_sha']=='366ee6ef8f9d2b9b57331dfd26b6f908c0289851'
assert candidate['version']==published['version']=='0.6.24-dev'
producer_hashes = read('producer-hashes.json')
for path, expected in producer_hashes.items(): assert hashlib.sha256((root/path).read_bytes()).hexdigest()==expected
go_hash = 'sha256:'+next(value for path,value in producer_hashes.items() if path=='tools/go')

summaries={}
observations={}
for arm in ['baseline','constrained']:
    fresh=read(arm+'-fresh.stdout.json')
    construction=fresh['construction']
    assert construction['original_source_sha256']==file_sha(arm+'.gooo')
    assert construction['stage']=='COMPLETE' and construction['decision']=='COMPLETE_FINITE'
    assert construction['candidate_space']=='4' and construction['program_budget']==4
    assert construction['initial']['model']['loaded'] is False
    assert construction['selected_source']==(root/(arm+'-fresh/selected.gooo')).read_text()
    assert (root/(arm+'-fresh/original.gooo')).read_bytes()==(root/(arm+'.gooo')).read_bytes()
    preparation=construction['initial']['preparations'][0]
    assert preparation['input_source_sha256']==file_sha(arm+'.gooo')
    generation=preparation['generation'];report=generation['report'];paths=report['body_paths']
    assert report['activity_id']=='byteguard://activity/byte-or-reject'
    assert report['compiler_source_sha']==revision
    assert report['source_digest']==sha(generation['gooo_source'].encode())
    assert report['generated_digest']==sha(generation['source'].encode())
    assert paths['original_source_sha256']==file_sha(arm+'.gooo')
    selection=paths['search']['selection']
    assert selection['local_model_predictions']==selection['external_provider_calls']==0
    assert selection['external_provider_calls_known'] is True
    chosen=construction['attempts'][construction['selected_attempt']]['path_candidates'][0]
    assert chosen['choices']==selection['choices']
    assert chosen['selected_source_sha256']==file_sha(arm+'-fresh/selected.gooo')
    actual_observed=read(arm+'-observer.stdout.json')
    assert [row['input'] for row in actual_observed]==inputs
    for row in actual_observed:
        assert type(row['input']) is int and type(row['actual']) is int and type(row['condition']) is bool
        expected=by_input[row['input']]
        assert row['actual']==expected['expected']
        assert row['condition'] is expected['inside' if arm=='baseline' else 'outside']
    observations[arm]={row['input']:row for row in actual_observed}
    assert (root/(arm+'-observer/generated.go')).read_bytes()==(root/(arm+'-fresh/generated.go')).read_bytes()
    if arm=='baseline':
        assert 'conditions' not in paths and 'conditions' not in chosen
        assert chosen['mask']==1
    else:
        assert chosen['mask']==2
        for conditions in [paths['conditions'],chosen['conditions']]:
            assert conditions['declared']==conditions['passed']==13 and conditions['not_reached']==0
            assert conditions['selected_tree_sha256']==conditions['emitted_tree_sha256']
            assert [(c['case']['input'],c['case']['expected']) for c in conditions['results']]==condition_cases
            for case in conditions['results']:
                expected=by_input[case['case']['input']]
                assert case['observation']['reached'] is True
                assert case['observation']['value'] is expected['outside']
                assert case['output']['type']=='Int' and case['output'].get('int',0)==expected['expected']
        rejected=[attempt for attempt in paths['search']['attempts'] if attempt['choice_mask']==1][0]
        assert rejected['status']=='CONDITION_REJECTED'
        assert rejected['training_cases_passed']==rejected['training_cases_total']==13
        assert len(rejected['condition_results'])==13
        for case in rejected['condition_results']:
            expected=by_input[case['case']['input']]
            assert case['observation']['value'] is expected['inside']
            assert case['case']['expected'] is expected['outside']
            assert case['passed'] is False

    evaluation_counts=[]
    replay_inputs=[]
    for label, batch, replay in [(arm+'-fresh',0,False)]+[(f'{arm}-replay-{i}',i,True) for i in range(3)]:
        payload=read(label+'.stdout.json');evaluation=payload['evaluation'];runtime=evaluation['runtime']
        cases=read(f'evaluation-{batch}.json')['cases']
        assert payload['generated_now'] is (not replay)
        assert evaluation['construction_replayed'] is replay and evaluation['new_model_calls']==0
        assert payload['construction']==construction
        assert runtime['stage']=='COMPLETE' and runtime['model_calls']==0
        assert runtime['producer_source_sha']==revision and runtime['go_tool_sha256']==go_hash
        assert runtime['go_version'].startswith('go version go1.27.2 ')
        assert runtime['original_source_sha256']==runtime['selected_source_sha256']==file_sha(label+'/selected.gooo')
        assert runtime['generated_sha256']==file_sha(label+'/generated.go')
        assert runtime['driver_sha256']==file_sha(label+'/main.go')
        assert (root/(label+'/selected.gooo')).read_bytes()==(root/(arm+'-fresh/selected.gooo')).read_bytes()
        assert (root/(label+'/original.gooo')).read_bytes()==(root/(arm+'.gooo')).read_bytes()
        assert read(label+'/construction.json')==read(arm+'-fresh/construction.json')
        assert runtime['finite_passed']==runtime['finite_total']==len(cases)
        assert len(runtime['runs'])==2
        if replay:
            assert runtime['artifact']['reused'] and runtime['artifact']['executable_verified']
            assert runtime['toolchain_reference']['reused'] and runtime['toolchain_reference']['bytes_verified'] and runtime['toolchain_reference']['build_info_verified']
            processes=[runtime['artifact']['source_build'],runtime['toolchain_reference']['source_check']]+runtime['runs']
        else:
            processes=[runtime['toolchain'],runtime['build']]+runtime['runs']
        for process in processes:
            assert process['started'] and process['completed'] and process['exit_code']==0
            assert not process['canceled'] and not process['timed_out'] and not process['output_truncated']
        assert len(runtime['traces'])==len(cases)
        for i,(trace,case) in enumerate(zip(runtime['traces'],cases)):
            assert trace['case_index']==i and len(trace['deliveries'])==1
            delivery=trace['deliveries'][0];value=case['inputs']['Main'];expected=by_input[value]['expected']
            assert delivery['activity_id']=='byteguard://activity/main'
            assert type(delivery['input']) is int and type(delivery['actual']) is int
            assert delivery['input']==value and delivery['actual']==expected==case['expected']['Main']==delivery['expected']
            if replay: replay_inputs.append(value)
        for item in walk(payload):
            for key in ['model_calls','new_model_calls','local_model_predictions','external_provider_calls']:
                if key in item: assert item[key]==0
        evaluation_counts.append({'label':label,'cases':len(cases),'native_process_runs':2,'replayed':replay})
    assert replay_inputs==inputs
    outside_matches=sum(row['condition'] is by_input[row['input']]['outside'] for row in actual_observed)
    summaries[arm]={'selected_mask':chosen['mask'],'choices':chosen['choices'],'source_sha256':file_sha(arm+'.gooo'),'selected_source_sha256':file_sha(arm+'-fresh/selected.gooo'),'generated_go_sha256':file_sha(arm+'-fresh/generated.go'),'construction_receipt_sha256':file_sha(arm+'-fresh/construction.json'),'evaluation_runs':evaluation_counts,'unique_replayed_inputs':268,'unique_independently_observed_inputs':268,'independent_output_matches':268,'independent_outside_condition_matches':outside_matches,'source_condition_cases':13 if arm=='constrained' else 0,'model_calls':0}
assert all(observations['baseline'][x]['actual']==observations['constrained'][x]['actual'] for x in inputs)
assert all(observations['baseline'][x]['condition'] is not observations['constrained'][x]['condition'] for x in inputs)
commands=read('commands.json')['commands']
old=[command for command in commands if command['label']=='published-constrained'][0]
assert old['exit_code']==1 and 'unknown assembly field condition_case' in (root/'published-constrained.stderr.log').read_text()
result={'status':'PASS','engine_revision':revision,'candidate_build_identity':candidate,'published_build_identity':published,'source_pair_diff':'Only the 13 condition_case declarations differ.','oracle_inputs':268,'all_byte_values':256,'invalid_inputs':12,'source_case_overlap':13,'construction_case_overlap':4,'arms':summaries,'output_equivalence_observed':268,'opposite_conditions_observed':268,'constrained_rejected_output_equivalent_mask':1,'constrained_rejected_mask_output_cases_passed':13,'constrained_rejected_mask_condition_cases_failed':13,'published_syntax_probe':{'exit_code':1,'diagnostic':'unknown assembly field condition_case','raw_stderr_sha256':file_sha('published-constrained.stderr.log')},'limits':{'source_cases_max':128,'source_condition_cases_max':128,'caller_cases_per_file_max':128,'caller_case_file_bytes_max':32768,'source_choices_max':16,'attempts_max':64,'used_choices':2,'used_declared_combinations':4,'used_attempt_budget':4,'evaluation_batch_sizes':[128,128,12]},'scope':'Deliberately constructed finite ambiguity. A useful byte-domain sentinel guard research example; not a new standard-library port, general correctness proof, compiler bug, model understanding, or performance result. Native receipts and generated-Go AST observer checked independently against a Go int64 oracle. No source/engine/pin changes.'}
(root/'verification-report.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps({'status':result['status'],'arms':{arm:{k:v for k,v in data.items() if k in ['selected_mask','independent_output_matches','independent_outside_condition_matches']} for arm,data in summaries.items()},'published_exit':1}))
