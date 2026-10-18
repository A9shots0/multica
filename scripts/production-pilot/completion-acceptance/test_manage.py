"""Failure-injection tests. No live API, daemon, provider or credential access."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import manage as m


class SetupRecoveryTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root=Path(self.temp.name)
        self.state=root/'state'; self.state.mkdir()
        self.profile=root/'daemon'; self.profile.mkdir()
        for name,value in [('STATE',self.state),('PROFILE',self.profile),('ISSUE','44444444-4444-4444-8444-444444444444')]:
            p=patch.object(m,name,value);p.start();self.addCleanup(p.stop)
        for name,value in [('exact_host',lambda:None),('check_bound_binaries',lambda:None),('sha',lambda p:'a'*64),('owner_session',lambda:'fixture-owner-session')]:
            p=patch.object(m,name,value);p.start();self.addCleanup(p.stop)

    def test_partial_agent_response_resumes_without_duplicates(self):
        runtime='11111111-1111-4111-8111-111111111111'
        m.save('setup-journal.json',{'runtime_id':runtime,'runtime_owner_id':m.OWNER,'roles':[]})
        agents={};squads={};members={};failed=False
        def find(sql):
            if 'FROM agent ' in sql:
                return next((v for k,v in agents.items() if "name='"+k+"'" in sql),None)
            if 'FROM squad ' in sql:return squads.get('id')
            if 'FROM squad_member ' in sql:return next((v for k,v in members.items() if k in sql),None)
            raise AssertionError(sql)
        def api(method,path,body=None,token=None):
            nonlocal failed
            if method=='POST' and path=='/api/agents/':
                value=f'00000000-0000-4000-8000-{len(agents)+1:012d}'
                agents[body['name']]=value
                if 'Writer' in body['name'] and not failed:
                    failed=True;raise RuntimeError('Injected lost response after successful create')
                return {'id':value}
            if method=='POST' and path=='/api/squads/':
                self.assertFalse(squads);squads['id']='22222222-2222-4222-8222-222222222222';return squads
            if method=='POST' and path.endswith('/members'):
                members[body['member_id']]='33333333-3333-4333-8333-333333333333';return {}
            if method=='PUT':return {}
            raise AssertionError((method,path))
        with patch.object(m,'query',return_value='0'),patch.object(m,'one_id',side_effect=find),patch.object(m,'api',side_effect=api),patch.object(m,'pilot_runtime',return_value={'id':runtime}):
            with self.assertRaisesRegex(RuntimeError,'lost response'):m.stage_issue()
            self.assertEqual(len(agents),2)
            self.assertEqual(json.loads((self.state/'setup-journal.json').read_text())['pending'],'agent_writer')
            m.stage_issue()
            self.assertEqual(len(agents),3)
            self.assertEqual(len(members),2)
            m.stage_issue()
            self.assertEqual(len(agents),3)
            self.assertTrue((self.state/'authority-contract.json').exists())

    def test_owner_registers_offline_before_daemon_token(self):
        calls=[];registered=False
        def query(sql):
            if 'INSERT INTO daemon_token' in sql:
                self.assertTrue(registered);calls.append('scoped-token');return ''
            return '0'
        def api(method,path,body=None,token=None):
            nonlocal registered
            if method=='GET':return {'status':'todo','assignee_type':'member','assignee_id':m.OWNER}
            self.assertEqual(path,'/api/daemon/register')
            self.assertEqual(token,'fixture-owner-session')
            self.assertEqual(body['runtimes'][0]['status'],'offline')
            registered=True;calls.append('owner-offline');return []
        runtime={'id':'11111111-1111-4111-8111-111111111111','owner_id':m.OWNER}
        with patch.object(m,'query',side_effect=query),patch.object(m,'api',side_effect=api),patch.object(m,'pilot_runtime',return_value=runtime),patch.object(m,'private_dir',side_effect=lambda p:p.mkdir(exist_ok=True)):
            m.provision_daemon()
        self.assertEqual(calls,['owner-offline','scoped-token'])

    def test_cleanup_removes_local_credentials_despite_server_errors(self):
        m.save('setup-journal.json',{'runtime_id':'fixture-runtime','roles':[]})
        m.save('authority-contract.json',{})
        m.save('owner-session.json',{'token':'fake-only'})
        (self.profile/'config.json').write_text(json.dumps({'token':'fake-only'}))
        home=self.state/'provider-home';(home/'.claude').mkdir(parents=True)
        (home/'.claude/.credentials.json').write_text('non-secret fixture')
        with patch.object(m,'query',return_value='0'),patch.object(m,'authority',side_effect=RuntimeError('Injected revoke failure')),patch.object(m,'api',side_effect=RuntimeError('Injected deregister failure')),patch.object(m,'stop_pilot_process'):
            with self.assertRaisesRegex(RuntimeError,'cleanup is incomplete'):m.cleanup()
        self.assertFalse(home.exists())
        self.assertFalse((self.profile/'config.json').exists())
        self.assertFalse((self.state/'owner-session.json').exists())
        result=json.loads((self.state/'cleanup.json').read_text())
        self.assertEqual({e['step'] for e in result['errors']},{'runtime_receipts_revoked','runtime_offline'})

    def test_cleanup_rejects_literal_junction_before_resolve(self):
        home=self.state/'provider-home';home.mkdir()
        (home/'marker').write_text('retained')
        with patch.object(Path,'is_junction',return_value=True):
            with self.assertRaisesRegex(RuntimeError,'target is a link'):m.remove_provider_profile()
        self.assertTrue((home/'marker').exists())

    def test_runtime_owner_is_required(self):
        row={'id':'fixture','provider':'claude','owner_id':None,'visibility':'private','status':'online'}
        with patch.object(m,'query',return_value=json.dumps([row])):
            with self.assertRaisesRegex(RuntimeError,'owner-bound'):m.pilot_runtime(True)



class BoundBinaryTests(unittest.TestCase):
    def test_changed_cli_or_authority_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);state=root/'state';state.mkdir()
            contract={'roles':[{'id':'role-a'},{'id':'role-b'},{'id':'role-c'}],'broker_sha256':'a'*64,'provider_sha256':'a'*64}
            bound={'issue_id':'fixture-issue','roles':['role-a','role-b','role-c'],'qualification_sha256':'a'*64,'broker_sha256':'a'*64,'cli_sha256':'a'*64,'authority_sha256':'a'*64}
            (state/'authority-contract.json').write_text(json.dumps(contract));(state/'bound-binaries.json').write_text(json.dumps(bound))
            with patch.object(m,'STATE',state),patch.object(m,'ROOT',root),patch.object(m,'ISSUE','fixture-issue'),patch.object(m,'qualify',lambda:None):
                for changed in [m.CLI,root/'server/bin/silv38-completion-acceptance-authority']:
                    with patch.object(m,'sha',side_effect=lambda path:'b'*64 if path==changed else 'a'*64):
                        with self.assertRaisesRegex(RuntimeError,'bound executable drift'):m.check_bound_binaries()

if __name__=='__main__':unittest.main()
