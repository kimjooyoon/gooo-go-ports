#!/usr/bin/env python3
"""Transport-only fixtures; no policy outputs, compiler, Go or subprocess use."""
import importlib.util
import io
import json
import pathlib
import unittest

HERE=pathlib.Path(__file__).resolve().parent
SPEC=importlib.util.spec_from_file_location('retry_after_request_inputs',HERE/'request_inputs.py')
GATE=importlib.util.module_from_spec(SPEC);SPEC.loader.exec_module(GATE)


class RequestInputTests(unittest.TestCase):
    def request(self):
        return {'header':'60','header_present':True,'elapsed_seconds':59,
                'max_wait_seconds':3600,'request_binding_current':True,'guidance_applicable':True}

    def raw(self,request=None):
        return json.dumps(self.request() if request is None else request,
                          ensure_ascii=False,separators=(',',':')).encode('utf-8')

    def bound(self,raw):
        result=json.loads(GATE.bind_request(raw).decode('utf-8'))
        self.assertEqual(set(result),{'schema','inputs'})
        self.assertEqual(result['schema'],'gooo/body-composition-inputs/v1')
        self.assertEqual(len(result['inputs']),1)
        self.assertEqual(set(result['inputs'][0]),{GATE.ENTRY+'.input'+str(i) for i in range(6)})
        return result['inputs'][0]

    def refuse(self,raw):
        with self.assertRaises(GATE.InputRefusal):GATE.bind_request(raw)
        stdout=io.BytesIO();stderr=io.StringIO()
        self.assertEqual(GATE.main([],stdin=io.BytesIO(raw),stdout=stdout,stderr=stderr),1)
        self.assertEqual(stdout.getvalue(),b'');self.assertTrue(stderr.getvalue())

    def test_starter_and_exact_six_original_values(self):
        raw=(HERE/'request.json').read_bytes();request=GATE.parse_request(raw)
        self.assertEqual(request,self.request())
        self.assertEqual(self.bound(raw),{GATE.ENTRY+'.input'+str(i):request[name]
                                         for i,name in enumerate(GATE.FIELDS)})

    def test_semantically_invalid_facts_and_full_int64_endpoints_are_preserved(self):
        for elapsed,maximum in [(-1,86401),(GATE.MIN_INT64,GATE.MAX_INT64),
                                (GATE.MAX_INT64,GATE.MIN_INT64),(0,-1)]:
            with self.subTest(elapsed=elapsed,maximum=maximum):
                request=self.request();request.update(header='-1',header_present=False,
                    elapsed_seconds=elapsed,max_wait_seconds=maximum,
                    request_binding_current=False,guidance_applicable=False)
                row=self.bound(self.raw(request))
                self.assertEqual([row[GATE.ENTRY+'.input'+str(i)] for i in range(6)],
                                 [request[name] for name in GATE.FIELDS])
                self.assertEqual(set(row),{GATE.ENTRY+'.input'+str(i) for i in range(6)})

    def test_header_text_is_not_trimmed_normalized_or_policy_parsed(self):
        for header in ('',' +00060 ','-0','Mon, 20 Oct 2025 12:00:00 GMT','\x00\r\n','e\u0301','\u00e9','{}[]"\\'):
            with self.subTest(header=header):
                request=self.request();request['header']=header
                self.assertEqual(self.bound(self.raw(request))[GATE.ENTRY+'.input0'],header)

    def test_utf8_byte_cap_and_unicode_scalar_boundaries(self):
        for header in ('a'*64,'가'*21,'😀'*16,'\U00010000'*16,'\uffff'*21):
            with self.subTest(header=header):
                request=self.request();request['header']=header
                self.assertEqual(self.bound(self.raw(request))[GATE.ENTRY+'.input0'],header)
        for header in ('a'*65,'가'*22,'😀'*17):
            with self.subTest(header=header):
                request=self.request();request['header']=header;self.refuse(self.raw(request))

    def test_escaped_valid_pair_and_lone_surrogates(self):
        raw=self.raw().replace(b'"60"',b'"\\ud83d\\ude00"',1)
        self.assertEqual(self.bound(raw)[GATE.ENTRY+'.input0'],'😀')
        for escaped in (b'"\\ud800"',b'"\\udfff"',b'"\\ud800x"',b'"\\ud800\\ud800"'):
            with self.subTest(escaped=escaped):self.refuse(self.raw().replace(b'"60"',escaped,1))

    def test_invalid_utf8_raw_control_and_bom_refuse(self):
        for raw in (self.raw().replace(b'"60"',b'"\xff"',1),
                    self.raw().replace(b'"60"',b'"\xed\xa0\x80"',1),
                    self.raw().replace(b'"60"',b'"\x00"',1),b'\xef\xbb\xbf'+self.raw()):
            with self.subTest(raw=raw):self.refuse(raw)

    def test_exact_keys_missing_unknown_and_decoded_duplicates_refuse(self):
        for name in GATE.FIELDS:
            with self.subTest(missing=name):
                request=self.request();del request[name];self.refuse(self.raw(request))
        request=self.request();request['unknown']=True;self.refuse(self.raw(request))
        for name in GATE.FIELDS:
            escaped='\\u%04x'%ord(name[0])+name[1:]
            duplicate=(self.raw()[:-1]+b','+json.dumps(escaped).encode().replace(b'\\\\u',b'\\u')+b':'+json.dumps(self.request()[name]).encode()+b'}')
            with self.subTest(duplicate=name):self.refuse(duplicate)
        self.refuse(self.raw()[:-1]+b',"header":"60"}')
        # One escaped spelling is accepted; only duplicated decoded names refuse.
        raw=self.raw().replace(b'"header"',b'"\\u0068eader"',1)
        self.assertEqual(self.bound(raw)[GATE.ENTRY+'.input0'],'60')

    def test_types_boolean_integer_separation_and_nonfinite_refuse(self):
        for name,values in [('header',[None,False,0,[]]),
                            ('elapsed_seconds',[True,False,'59',None]),
                            ('max_wait_seconds',[True,False,'3600',None]),
                            *[(name,[0,1,'true',None]) for name in ('header_present','request_binding_current','guidance_applicable')]]:
            for value in values:
                with self.subTest(name=name,value=value):
                    request=self.request();request[name]=value;self.refuse(self.raw(request))
        for name,original in ((b'elapsed_seconds',b'59'),(b'max_wait_seconds',b'3600')):
            for token in (b'1.0',b'1e0',b'NaN',b'Infinity',b'-Infinity',b'9223372036854775808',b'-9223372036854775809'):
                with self.subTest(name=name,token=token):
                    self.refuse(self.raw().replace(b'"'+name+b'":'+original,b'"'+name+b'":'+token,1))

    def test_flat_shape_depth_trailing_and_malformed_refuse(self):
        for raw in (b'',b' ',b'[]',b'null',b'true',b'0',b'"request"',self.raw()+b'{}',
                    self.raw()+b'true',self.raw()[:-1],b'['*1000+b']'*1000,
                    self.raw().replace(b'"60"',b'{"nested":"60"}',1),
                    self.raw().replace(b'"60"',b'["60"]',1)):
            with self.subTest(raw=raw[:50]):self.refuse(raw)

    def test_raw_byte_cap_is_exact_and_read_is_bounded_once(self):
        raw=self.raw();padded=b' '*(GATE.MAX_RAW_BYTES-len(raw))+raw
        self.assertEqual(len(padded),4096);self.bound(padded)
        self.refuse(b' '+padded)
        class Spy(io.BytesIO):
            def __init__(self,value):super().__init__(value);self.calls=[]
            def read(self,size=-1):self.calls.append(size);return super().read(size)
        source=Spy(padded);stdout=io.BytesIO();stderr=io.StringIO()
        self.assertEqual(GATE.main(['-'],stdin=source,stdout=stdout,stderr=stderr),0)
        self.assertEqual(source.calls,[4097]);self.assertEqual(stderr.getvalue(),'')
        self.assertEqual(json.loads(stdout.getvalue())['schema'],'gooo/body-composition-inputs/v1')
        source=Spy(b'x'*10000);stdout=io.BytesIO();stderr=io.StringIO()
        self.assertEqual(GATE.main([],stdin=source,stdout=stdout,stderr=stderr),1)
        self.assertEqual(source.calls,[4097]);self.assertEqual(source.tell(),4097);self.assertEqual(stdout.getvalue(),b'')

    def test_file_cli_and_missing_file_have_no_diagnostic_on_success_stdout(self):
        stdout=io.BytesIO();stderr=io.StringIO()
        self.assertEqual(GATE.main([str(HERE/'request.json')],stdout=stdout,stderr=stderr),0)
        self.assertEqual(stderr.getvalue(),'');self.assertEqual(stdout.getvalue().count(b'\n'),1)
        self.bound(self.raw());self.assertEqual(json.loads(stdout.getvalue())['inputs'][0][GATE.ENTRY+'.input2'],59)
        stdout=io.BytesIO();stderr=io.StringIO()
        self.assertEqual(GATE.main([str(HERE/'does-not-exist.json')],stdout=stdout,stderr=stderr),1)
        self.assertEqual(stdout.getvalue(),b'');self.assertTrue(stderr.getvalue())


if __name__ == '__main__':
    unittest.main()
