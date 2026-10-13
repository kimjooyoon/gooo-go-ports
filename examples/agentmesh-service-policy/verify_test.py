"""Pure evidence-checker regressions; these fixtures are not native observations.

The small fixture projects real retained research rows and equal execute/replay
traces. Repetition and mutation below test verifier invariants only. No compiler,
Go build, network request or workspace execution runs from this test module.
"""

import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest

from verification_checks import check_native_rows, check_workspace_trace


ROOT = Path(__file__).resolve().parent
FIXTURE = json.loads((ROOT / 'testdata/verifier-fixture.json').read_text())


def native_inputs():
    """Repeat one fixture row to isolate the exact 63/16 cardinality contract."""
    bindings = copy.deepcopy(FIXTURE['expected_bindings'])
    matcher = dict(copy.deepcopy(FIXTURE['native']['matcher']), bindings=bindings)
    impact = dict(copy.deepcopy(FIXTURE['native']['impact']), bindings=bindings)
    row = dict(matcher['facts'], matched=matcher['matched'])
    changed = dict(impact['facts'], want=impact['decision'])
    return [copy.deepcopy(matcher) for _ in range(63)], [copy.deepcopy(impact) for _ in range(16)], \
        [copy.deepcopy(row) for _ in range(63)], [copy.deepcopy(changed) for _ in range(16)], bindings


def workspace_check(prefix, context):
    check_workspace_trace(prefix, context['trace'], context['index'], context['case'],
                          context['program'], context['body_report'])


class NativeEvidenceChecks(unittest.TestCase):
    def test_real_projection_fixture_and_valid_cardinalities(self):
        self.assertIs(FIXTURE['fixture_only'], True)
        self.assertEqual(FIXTURE['schema'], 'gooo/agentmesh-verifier-test-fixture/v1')
        self.assertEqual(len(FIXTURE['origins']), 8)
        self.assertTrue(all(len(origin['sha256']) == 64 for origin in FIXTURE['origins']))
        check_native_rows(*native_inputs())

    def test_truncated_empty_and_extra_native_arrays_refused(self):
        for position, count in [(0, 63), (1, 16)]:
            for size in [0, count - 1, count + 1]:
                with self.subTest(position=position, size=size):
                    args = list(native_inputs())
                    args[position] = (args[position] * 2)[:size]
                    with self.assertRaises(AssertionError):
                        check_native_rows(*args)

    def test_native_array_shape_and_row_fields_refused(self):
        for position in [0, 1]:
            with self.subTest(position=position, mutation='tuple'):
                args = list(native_inputs())
                args[position] = tuple(args[position])
                with self.assertRaises(AssertionError):
                    check_native_rows(*args)
            with self.subTest(position=position, mutation='extra-row-field'):
                args = list(native_inputs())
                args[position][-1]['extra'] = 'unexpected'
                with self.assertRaises(AssertionError):
                    check_native_rows(*args)

    def test_each_impact_raw_fact_refused_when_altered(self):
        for field in ['before', 'after', 'req_type', 'req_name']:
            for mutation in ['value', 'missing', 'wrong-type']:
                with self.subTest(field=field, mutation=mutation):
                    args = list(native_inputs())
                    facts = args[1][-1]['facts']
                    if mutation == 'missing':
                        del facts[field]
                    else:
                        facts[field] = 'different' if mutation == 'value' else False
                    with self.assertRaises(AssertionError):
                        check_native_rows(*args)
        args = list(native_inputs())
        args[1][-1]['facts']['extra'] = 'unexpected'
        with self.assertRaises(AssertionError):
            check_native_rows(*args)

    def test_native_boolean_zero_one_refused_even_when_equal(self):
        for field in ['matched', 'before_match', 'after_match']:
            for number in [0, 1]:
                with self.subTest(field=field, number=number):
                    args = list(native_inputs())
                    if field == 'matched':
                        args[2][-1]['matched'] = bool(number)
                        args[0][-1]['matched'] = number
                    else:
                        args[3][-1]['want'][field] = bool(number)
                        args[1][-1]['decision'][field] = number
                    with self.assertRaises(AssertionError):
                        check_native_rows(*args)

    def test_native_decision_string_and_binding_changes_refused(self):
        args = list(native_inputs())
        args[1][-1]['decision']['change'] = 1
        with self.assertRaises(AssertionError):
            check_native_rows(*args)
        for position in [0, 1]:
            with self.subTest(position=position):
                args = list(native_inputs())
                args[position][-1]['bindings']['generation_sha256'] = 'sha256:changed'
                with self.assertRaises(AssertionError):
                    check_native_rows(*args)


class WorkspaceEvidenceChecks(unittest.TestCase):
    def reject_each_phase(self, prefix, mutation, change):
        for phase in FIXTURE['workspace'][prefix]['phases']:
            with self.subTest(prefix=prefix, phase=phase, mutation=mutation):
                context = copy.deepcopy(FIXTURE['workspace'][prefix])
                change(context)
                with self.assertRaises(AssertionError):
                    workspace_check(prefix, context)

    def test_real_execute_and_replay_projections_accepted(self):
        for prefix in ['matcher', 'impact']:
            context = FIXTURE['workspace'][prefix]
            self.assertEqual(context['phases'], ['execute', 'replay'])
            for phase in context['phases']:
                with self.subTest(prefix=prefix, phase=phase):
                    workspace_check(prefix, copy.deepcopy(context))

    def test_trace_shape_case_index_and_activity_identity_refused(self):
        for prefix in ['matcher', 'impact']:
            for mutation, change in [
                ('index', lambda c: c['trace'].__setitem__('case_index', 1)),
                ('bool-index', lambda c: c['trace'].__setitem__('case_index', False)),
                ('no-delivery', lambda c: c['trace'].__setitem__('deliveries', [])),
                ('extra-delivery', lambda c: c['trace']['deliveries'].append(copy.deepcopy(c['trace']['deliveries'][0]))),
                ('activity-id', lambda c: c['trace']['deliveries'][0].__setitem__('activity_id', 'gooo://wrong/activity')),
                ('passed-number', lambda c: c['trace']['deliveries'][0].__setitem__('passed', 1)),
            ]:
                self.reject_each_phase(prefix, mutation, change)

    def test_workspace_entry_and_bound_export_contract_refused(self):
        for prefix in ['matcher', 'impact']:
            for field in ['package_path', 'activity', 'lowered_name']:
                self.reject_each_phase(prefix, 'entry-' + field,
                                       lambda c, f=field: c['program']['entry'].__setitem__(f, 'wrong'))
            for field in ['activity', 'activity_id', 'output_type']:
                self.reject_each_phase(prefix, 'report-' + field,
                                       lambda c, f=field: c['body_report'].__setitem__(f, 'wrong'))
            for field in ['id', 'name', 'kind', 'output_type']:
                self.reject_each_phase(prefix, 'export-' + field,
                                       lambda c, f=field: c['program']['workspace_image']['packages'][0]['exports'][0].__setitem__(f, 'wrong'))
            self.reject_each_phase(prefix, 'input-type',
                                   lambda c: c['program']['workspace_image']['packages'][0]['exports'][0]['input_types'].__setitem__(-1, 'gooo://wrong/text'))

    def test_input_port_entity_value_type_and_roster_refused(self):
        for prefix in ['matcher', 'impact']:
            for field, value in [('port', 'input0'), ('entity_id', 'gooo://wrong/text'), ('value', False)]:
                self.reject_each_phase(prefix, 'last-input-' + field,
                                       lambda c, f=field, v=value: c['trace']['deliveries'][0]['inputs'][-1].__setitem__(f, v))
            for mutation, change in [
                ('missing-input', lambda c: c['trace']['deliveries'][0]['inputs'].pop()),
                ('extra-input', lambda c: c['trace']['deliveries'][0]['inputs'].append(copy.deepcopy(c['trace']['deliveries'][0]['inputs'][0]))),
                ('reordered-input', lambda c: c['trace']['deliveries'][0]['inputs'].reverse()),
                ('extra-input-key', lambda c: c['trace']['deliveries'][0]['inputs'][-1].__setitem__('type_id', 'unexpected')),
            ]:
                self.reject_each_phase(prefix, mutation, change)

    def test_workspace_boolean_zero_one_refused_even_when_equal(self):
        for prefix, fields in [('matcher', [None]), ('impact', ['before_match', 'after_match'])]:
            for field in fields:
                for number in [0, 1]:
                    def change(context, f=field, n=number):
                        delivery = context['trace']['deliveries'][0]
                        expected = next(iter(context['case']['expected']))
                        if f is None:
                            context['case']['expected'][expected] = bool(n)
                            delivery['expected'] = bool(n)
                            delivery['actual'] = n
                        else:
                            context['case']['expected'][expected][f] = bool(n)
                            delivery['expected'][f] = bool(n)
                            delivery['actual'][f] = n
                            for item in delivery['actual_fields']:
                                if item['name'] == f:
                                    item['value'] = bool(n)
                    self.reject_each_phase(prefix, str(field) + '-' + str(number), change)

    def test_impact_field_id_name_value_and_roster_refused(self):
        for index in range(3):
            for field, value in [('id', 'gooo://wrong/field'), ('name', 'wrong'), ('value', 1)]:
                self.reject_each_phase('impact', str(index) + '-' + field,
                                       lambda c, i=index, f=field, v=value: c['trace']['deliveries'][0]['actual_fields'][i].__setitem__(f, v))
        for mutation, change in [
            ('missing-field', lambda c: c['trace']['deliveries'][0]['actual_fields'].pop()),
            ('extra-field', lambda c: c['trace']['deliveries'][0]['actual_fields'].append(copy.deepcopy(c['trace']['deliveries'][0]['actual_fields'][0]))),
            ('reordered-fields', lambda c: c['trace']['deliveries'][0]['actual_fields'].reverse()),
        ]:
            self.reject_each_phase('impact', mutation, change)

    def test_impact_record_schema_identity_field_types_and_roster_refused(self):
        for field in ['id', 'name']:
            self.reject_each_phase('impact', 'record-' + field,
                                   lambda c, f=field: c['body_report']['record_types'][0].__setitem__(f, 'wrong'))
        for index in range(3):
            for field in ['id', 'name', 'type_id']:
                self.reject_each_phase('impact', 'schema-' + str(index) + '-' + field,
                                       lambda c, i=index, f=field: c['body_report']['record_types'][0]['fields'][i].__setitem__(f, 'wrong'))
        self.reject_each_phase('impact', 'missing-schema-field',
                               lambda c: c['body_report']['record_types'][0]['fields'].pop())
        self.reject_each_phase('impact', 'extra-schema-field',
                               lambda c: c['body_report']['record_types'][0]['fields'].append(copy.deepcopy(c['body_report']['record_types'][0]['fields'][0])))


class OptimizedPythonRefusal(unittest.TestCase):
    def test_startup_refuses_optimization_before_help_or_native_work(self):
        for script in ['generate.py', 'verify.py']:
            for mode in ['-O', 'PYTHONOPTIMIZE']:
                with self.subTest(script=script, mode=mode):
                    env = os.environ.copy()
                    env['PYTHONDONTWRITEBYTECODE'] = '1'
                    env.pop('PYTHONOPTIMIZE', None)
                    command = [sys.executable]
                    if mode == '-O':
                        command.append('-O')
                    else:
                        env['PYTHONOPTIMIZE'] = '1'
                    command += [str(ROOT / script), '--help']
                    result = subprocess.run(command, cwd=ROOT, env=env, capture_output=True,
                                            text=True, timeout=10)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('optimization', result.stderr.lower())
                    self.assertEqual(result.stdout, '')


# Synthetic fixtures check verifier rejection paths only; verify.py checks real
# native execute/replay evidence separately. No compiler output is forged here.
from verification_checks import (check_native_refinement, check_refinement_decision,
                                 check_witness_oracle, REFINEMENT_FIELDS)
REFINEMENT_CASES = json.loads((ROOT / 'refinement-cases.json').read_text())

def refinement_inputs():
    bindings = copy.deepcopy(FIXTURE['expected_bindings'])
    cases = copy.deepcopy(REFINEMENT_CASES)
    actual = [{'facts':{k:c[k] for k in ['before','after']},'decision':copy.deepcopy(c['want']),
               'bindings':copy.deepcopy(bindings)} for c in cases]
    return actual,cases,bindings

def refinement_workspace():
    c=copy.deepcopy(FIXTURE['workspace']['impact'])
    package='app/service-grant-refinement';activity='AnalyzeServiceGrant';entry=package+':'+activity
    aid='gooo://service-refinement/analyze';eid='gooo://service-refinement/decision/v1'
    c['program']['entry'].update(package_path=package,activity=activity)
    c['program']['workspace_image']['packages']=[{'path':package,'exports':[{'kind':'activity','name':activity,'id':aid,'input_types':['gooo://std/text']*2,'output_type':eid}]}]
    c['body_report'].update(activity_id=aid,output_type='GrantRefinement',record_types=[{'name':'GrantRefinement','id':eid,'fields':[{'name':n,'id':fid,'type_id':tid} for n,(fid,tid,_) in REFINEMENT_FIELDS.items()]}])
    row=next(r for r in REFINEMENT_CASES if r['name']=='suffix-to-prefix-second-seed')
    c['case']={'inputs':{entry+'.input0':row['before'],entry+'.input1':row['after']},'expected':{entry:copy.deepcopy(row['want'])}}
    c['trace']={'case_index':0,'deliveries':[{'activity_id':aid,'inputs':[{'port':'input0','entity_id':'gooo://std/text','value':row['before']},{'port':'input1','entity_id':'gooo://std/text','value':row['after']}],
      'actual':copy.deepcopy(row['want']),'expected':copy.deepcopy(row['want']),'passed':True,
      'actual_fields':[{'id':fid,'name':n,'value':row['want'][n]} for n,(fid,_,_) in REFINEMENT_FIELDS.items()]}]}
    return c

class RefinementEvidenceChecks(unittest.TestCase):
    def test_native_complete_contract_accepted(self):check_native_refinement(*refinement_inputs())
    def test_native_shape_facts_bindings_and_decision_mutations_refused(self):
        for count in [0,100,102]:
            args=list(refinement_inputs());args[0]=(args[0]*2)[:count]
            with self.assertRaises(AssertionError):check_native_refinement(*args)
        for field in ['before','after']:
            for value in ['changed',False,1,None]:
                with self.subTest(field=field,value=value):
                    args=list(refinement_inputs());args[0][-1]['facts'][field]=value
                    with self.assertRaises(AssertionError):check_native_refinement(*args)
        for field,(_,_,kind) in REFINEMENT_FIELDS.items():
            for mutation in ['wrong','type','missing']:
                with self.subTest(field=field,mutation=mutation):
                    args=list(refinement_inputs());d=args[0][-1]['decision']
                    if mutation=='missing':del d[field]
                    else:d[field]=(not d[field] if kind is bool else 'changed') if mutation=='wrong' else (int(d[field]) if kind is bool else False)
                    with self.assertRaises(AssertionError):check_native_refinement(*args)
        for field in ['compiler_source_sha','generation_sha256','generation_origin','assets']:
            args=list(refinement_inputs());args[0][-1]['bindings'][field]='changed'
            with self.assertRaises(AssertionError):check_native_refinement(*args)
        for target in ['facts','decision','bindings']:
            args=list(refinement_inputs());args[0][-1][target]['extra']=True
            with self.assertRaises(AssertionError):check_native_refinement(*args)
    def test_self_consistent_but_contradictory_decisions_refused(self):
        for status in ['SUPPORTED_SUBSET','WIDENING_WITNESS','UNPROVEN']:
            d=next((copy.deepcopy(r['want']) for r in REFINEMENT_CASES if r['want']['status']==status),dict(status='UNPROVEN',rule_id='',supported_subset=False,has_witness=False,witness_type='',witness_name='',before_match=False,after_match=False))
            check_refinement_decision(d,d)
            for field in REFINEMENT_FIELDS:
                bad=copy.deepcopy(d);bad[field]=not bad[field] if type(bad[field]) is bool else ('INVALID' if field in ['status','rule_id'] else ('x' if not bad[field] else ''))
                with self.subTest(status=status,field=field):
                    with self.assertRaises(AssertionError):check_refinement_decision(bad,bad)
        witness=copy.deepcopy(next(r['want'] for r in REFINEMENT_CASES if r['want']['has_witness']))
        for field in ['witness_type','witness_name']:
            for value in ['', 'é', 'x/y', 'a'*129]:
                bad=copy.deepcopy(witness);bad[field]=value
                with self.assertRaises(AssertionError):check_refinement_decision(bad,bad)
    def test_workspace_all_eight_typed_fields_and_identity(self):
        workspace_check('refinement',refinement_workspace());mutations=[]
        for i in range(8):
            for key,value in [('id','wrong'),('name','wrong'),('value',None)]:mutations.append((f'field-{i}-{key}',lambda c,i=i,k=key,v=value:c['trace']['deliveries'][0]['actual_fields'][i].__setitem__(k,v)))
            for key in ['id','name','type_id']:mutations.append((f'schema-{i}-{key}',lambda c,i=i,k=key:c['body_report']['record_types'][0]['fields'][i].__setitem__(k,'wrong')))
        mutations += [('reordered',lambda c:c['trace']['deliveries'][0]['actual_fields'].reverse()),('missing',lambda c:c['trace']['deliveries'][0]['actual_fields'].pop()),('extra',lambda c:c['trace']['deliveries'][0]['actual_fields'].append(c['trace']['deliveries'][0]['actual_fields'][0])),('activity',lambda c:c['trace']['deliveries'][0].__setitem__('activity_id','wrong')),('entity',lambda c:c['trace']['deliveries'][0]['inputs'][1].__setitem__('entity_id','wrong')),('port',lambda c:c['trace']['deliveries'][0]['inputs'][1].__setitem__('port','input0')),('raw-fact',lambda c:c['trace']['deliveries'][0]['inputs'][1].__setitem__('value','changed')),('record-id',lambda c:c['body_report']['record_types'][0].__setitem__('id','wrong')),('record-name',lambda c:c['body_report']['record_types'][0].__setitem__('name','wrong')),('entry-package',lambda c:c['program']['entry'].__setitem__('package_path','wrong')),('entry-activity',lambda c:c['program']['entry'].__setitem__('activity','wrong'))]
        for phase in ['execute','replay']:
            for name,change in mutations:
                with self.subTest(phase=phase,mutation=name):
                    c=refinement_workspace();change(c)
                    with self.assertRaises(AssertionError):workspace_check('refinement',c)
    def test_witness_oracle_full_request_roster_and_results(self):
        revision='9fd1a41a15174df0aaef601c942410c2116bff91';requests=[]
        for row in REFINEMENT_CASES:
            d=row['want']
            if d['has_witness']:
                for side,want in [('before',False),('after',True)]:requests.append(dict(name=row['name']+'/'+side,pattern=row[side],req_type=d['witness_type'],req_name=d['witness_name'],want=want))
        actual=dict(schema='gooo/agentmesh-matcher-oracle/v1',go_version='go1.27.2',source_revision=revision,observations=[dict({k:r[k] for k in ['name','pattern','req_type','req_name']},matched=r['want']) for r in requests],impact_matches=[])
        check_witness_oracle(actual,requests,revision)
        for field in ['name','pattern','req_type','req_name','matched']:
            changed=copy.deepcopy(actual);changed['observations'][-1][field]=False if field=='matched' else 'wrong'
            with self.assertRaises(AssertionError):check_witness_oracle(changed,requests,revision)
        for field in ['source_revision','go_version','schema']:
            changed=copy.deepcopy(actual);changed[field]='wrong'
            with self.assertRaises(AssertionError):check_witness_oracle(changed,requests,revision)
        for count in [0,125,127]:
            changed=copy.deepcopy(actual);changed['observations']=(changed['observations']*2)[:count]
            with self.assertRaises(AssertionError):check_witness_oracle(changed,requests,revision)

from verification_checks import CHECKED, check_checked_decision, check_native_checked, check_domain_trace
CHECKED_CASES=json.loads((ROOT/'checked-cases.json').read_text())


def checked_context(kind):
    config=CHECKED[kind];entry=config['package']+':'+config['activity']
    row=next(r for r in CHECKED_CASES if r['api']==kind)
    c=refinement_workspace();c['program']['entry'].update(package_path=config['package'],activity=config['activity'])
    c['program']['workspace_image']['packages']=[{'path':config['package'],'exports':[{'kind':'activity','name':config['activity'],'id':config['activity_id'],'input_types':['gooo://std/text']*len(config['inputs']),'output_type':config['record_id']}]}]
    c['body_report'].update(activity_id=config['activity_id'],output_type=config['record'],record_types=[{'name':config['record'],'id':config['record_id'],'fields':[{'name':n,'id':fid,'type_id':tid}for n,(fid,tid,_)in config['fields'].items()]}])
    c['case']={'inputs':{entry+'.input'+str(i):row['facts'][k]for i,k in enumerate(config['inputs'])},'expected':{entry:copy.deepcopy(row['want'])}}
    c['trace']={'case_index':0,'deliveries':[{'activity_id':config['activity_id'],'inputs':[{'port':'input'+str(i),'entity_id':'gooo://std/text','value':row['facts'][k]}for i,k in enumerate(config['inputs'])],
      'actual':copy.deepcopy(row['want']),'expected':copy.deepcopy(row['want']),'passed':True,
      'actual_fields':[{'id':fid,'name':n,'value':row['want'][n]}for n,(fid,_,_)in config['fields'].items()]}]}
    return c


class CheckedDomainEvidenceChecks(unittest.TestCase):
    def test_all_checked_fixture_decisions_have_complete_typed_contracts(self):
        self.assertEqual(len(CHECKED_CASES),250)
        for r in CHECKED_CASES:check_checked_decision(r['api'],r['want'],r['want'])
        for kind in CHECKED:workspace_check('checked-'+kind,checked_context(kind))

    def test_each_checked_field_type_identity_and_raw_facts_refused(self):
        for kind,config in CHECKED.items():
            for i,(name,(_,_,typ)) in enumerate(config['fields'].items()):
                for where in ['actual','expected','actual_fields','schema']:
                    with self.subTest(kind=kind,field=name,where=where):
                        c=checked_context(kind);d=c['trace']['deliveries'][0]
                        if where in ['actual','expected']:d[where][name]=0 if typ is bool else False
                        elif where=='actual_fields':d['actual_fields'][i]['id']='gooo://wrong/field'
                        else:c['body_report']['record_types'][0]['fields'][i]['type_id']='wrong'
                        with self.assertRaises(AssertionError):workspace_check('checked-'+kind,c)
            for i,key in enumerate(config['inputs']):
                c=checked_context(kind);c['trace']['deliveries'][0]['inputs'][i]['value']='changed-but-same-decision'
                with self.assertRaises(AssertionError):workspace_check('checked-'+kind,c)

    def test_coherent_raw_downgrade_or_activity_entity_mismatch_refused(self):
        for kind in CHECKED:
            for mutation in ['entry','activity_id','record_id','missing-field','extra-field','reordered','bool-status']:
                with self.subTest(kind=kind,mutation=mutation):
                    c=checked_context(kind);d=c['trace']['deliveries'][0];e=c['program']['workspace_image']['packages'][0]['exports'][0]
                    if mutation=='entry':c['program']['entry'].update(package_path='go/google/agentmesh/api',activity='MatchServicePattern')
                    elif mutation=='activity_id':e['id']=d['activity_id']=c['body_report']['activity_id']='raw://downgrade'
                    elif mutation=='record_id':e['output_type']=c['body_report']['record_types'][0]['id']='gooo://std/boolean'
                    elif mutation=='missing-field':d['actual_fields'].pop()
                    elif mutation=='extra-field':d['actual_fields'].append(copy.deepcopy(d['actual_fields'][0]))
                    elif mutation=='reordered':d['actual_fields'].reverse()
                    else:d['actual']['domain_status']=True
                    with self.assertRaises(AssertionError):workspace_check('checked-'+kind,c)

    def test_invalid_sentinels_cannot_be_relabelled_as_normal_denial(self):
        for kind,config in CHECKED.items():
            base=copy.deepcopy(next(r['want']for r in CHECKED_CASES if r['api']==kind and r['want']['domain_status']=='INVALID_INPUT'))
            for name,(_,_,typ) in config['fields'].items():
                bad=copy.deepcopy(base)
                if name=='domain_status':bad[name]='VALID_INPUT'
                elif name=='invalid_field':bad[name]=''
                elif name=='change':bad[name]='UNCHANGED_DENY'
                elif name=='status':bad[name]='UNPROVEN'
                else:bad[name]=True if typ is bool else 'stale'
                with self.subTest(kind=kind,field=name):
                    with self.assertRaises(AssertionError):check_checked_decision(kind,bad,bad)

    def test_checked_native_exact_roster_bindings_and_facts(self):
        bindings=copy.deepcopy(FIXTURE['expected_bindings'])
        values=[dict(facts=copy.deepcopy(r['facts']),decision=copy.deepcopy(r['want']),bindings=copy.deepcopy(bindings))for r in CHECKED_CASES]
        check_native_checked(values,CHECKED_CASES,bindings)
        for mutation in ['missing','extra','facts','binding','decision-type']:
            with self.subTest(mutation=mutation):
                actual=copy.deepcopy(values)
                if mutation=='missing':actual.pop()
                elif mutation=='extra':actual.append(copy.deepcopy(actual[0]))
                elif mutation=='facts':actual[-1]['facts']['before']='changed'
                elif mutation=='binding':actual[-1]['bindings']['generation_origin']='unknown'
                else:actual[-1]['decision']['supported_subset']=0
                with self.assertRaises(AssertionError):check_native_checked(actual,CHECKED_CASES,bindings)

    def test_domain_scalar_port_identity_and_boolean_type(self):
        activity='ValidateServicePattern';entry='gooo/service-domain:'+activity;aid='gooo://service-domain/pattern'
        c=copy.deepcopy(FIXTURE['workspace']['matcher']);c['program']['entry'].update(package_path='gooo/service-domain',activity=activity)
        c['program']['workspace_image']['packages']=[{'path':'gooo/service-domain','exports':[{'name':activity,'kind':'activity','id':aid,'input_types':['gooo://std/text'],'output_type':'gooo://std/boolean'}]}]
        c['body_report'].update(activity_id=aid,input_type='string',output_type='bool');c['case']={'inputs':{entry:'mcp://a/path'},'expected':{entry:False}}
        c['trace']={'case_index':0,'deliveries':[dict(activity_id=aid,input='mcp://a/path',actual=False,expected=False,passed=True)]}
        def check(x):check_domain_trace(activity,x['trace'],0,x['case'],x['program'],x['body_report'])
        check(c)
        for mutation in ['actual','input','port','entity','entry','case-key','empty','absent']:
            d=copy.deepcopy(c)
            if mutation=='actual':d['trace']['deliveries'][0]['actual']=0
            elif mutation=='input':d['trace']['deliveries'][0]['input']='changed'
            elif mutation=='entry':d['program']['entry']['activity']='MatchServicePattern'
            elif mutation=='entity':d['program']['workspace_image']['packages'][0]['exports'][0]['input_types']=['gooo://wrong/text']
            elif mutation=='port':d['trace']['deliveries'][0]['inputs']=[{'port':'input0','entity_id':'gooo://std/text','value':'mcp://a/path'}]
            elif mutation=='case-key':d['case']['inputs']={entry+'.input0':'mcp://a/path'}
            elif mutation=='empty':d['trace']['deliveries'][0]['input']=''
            else:del d['trace']['deliveries'][0]['input']
            with self.assertRaises(AssertionError):check(d)
        c['case']['inputs'][entry]='';c['trace']['deliveries'][0]['input']='';check(c)



class FrozenDomainReferenceChecks(unittest.TestCase):
    def test_exact_baseline_and_coherently_rehashed_forgery_refusal(self):
        import hashlib
        from verification_checks import check_domain_reference
        original=json.loads((ROOT/'domain-reference.json').read_text());check_domain_reference(original)
        for mode in ['function','declarations','full-source','source-sha','baseline']:
            with self.subTest(mode=mode):
                r=copy.deepcopy(original)
                if mode=='function':
                    r['functions']['validType']='func validType(value string) bool { return true }'
                    r['function_sha256']['validType']=hashlib.sha256(r['functions']['validType'].encode()).hexdigest()
                elif mode=='declarations':
                    r['declarations']=r['declarations'].replace('128','129')
                    r['declarations_sha256']=hashlib.sha256(r['declarations'].encode()).hexdigest()
                elif mode=='full-source':
                    r['source']=r['source'].replace('128','129')
                    r['source_sha256']=hashlib.sha256(r['source'].encode()).hexdigest()
                    r['source_bytes']=len(r['source'].encode())
                elif mode=='source-sha':r['source_sha256']='0'*64
                else:r['baseline_commit']='0'*40
                with self.assertRaises(AssertionError):check_domain_reference(r)


if __name__ == '__main__':
    unittest.main()
