// Disposable fixture records against the dedicated local database only.
import pg from "pg";
import fs from "node:fs";
const url = process.env.DATABASE_URL;
if (!url || new URL(url).hostname !== "127.0.0.1" || new URL(url).port !== "55438" || new URL(url).pathname !== "/multica_review") throw Error("Use the isolated mika review database");
const db = new pg.Client({connectionString:url}); await db.connect();
const insert = async (sql, values=[]) => (await db.query(sql,values)).rows[0].id;
await db.query("BEGIN");
try {
 const user = await insert(`INSERT INTO "user" (name,email,onboarded_at) VALUES ('Local review owner','mika-review@example.test',now()) RETURNING id`);
 const ws = await insert(`INSERT INTO workspace (name,slug,description,issue_prefix) VALUES ('Mika local review','mika-local-review','Isolated acceptance fixtures. No live work or provider connections.','DEMO') RETURNING id`);
 await db.query(`INSERT INTO member (workspace_id,user_id,role) VALUES ($1,$2,'owner')`,[ws,user]);
 const runtime = await insert(`INSERT INTO agent_runtime (workspace_id,name,runtime_mode,provider,status,owner_id,last_seen_at,daemon_id,visibility) VALUES ($1,'Local fixture runtime','local','opencode','offline',$2,now(),'mika-review-fixture','private') RETURNING id`,[ws,user]);
 const agent = await insert(`INSERT INTO agent (workspace_id,name,description,runtime_mode,runtime_id,owner_id,visibility,permission_mode) VALUES ($1,'Mika','Local acceptance fixtures. No model worker is connected.','local',$2,$3,'workspace','private') RETURNING id`,[ws,runtime,user]);
 const session = await insert(`INSERT INTO chat_session (workspace_id,agent_id,creator_id,title,explicitly_created_at) VALUES ($1,$2,$3,'Three issues, one conversation',now()) RETURNING id`,[ws,agent,user]);
 const issueIds=[];
 for (const [idx,title] of ['Audit the content pipeline','Choose the compact preview','Review the mobile layout'].entries()) {
  const id = await insert(`INSERT INTO issue (workspace_id,title,status,creator_type,creator_id,assignee_type,assignee_id,number) VALUES ($1,$2,'in_progress','member',$3,'agent',$4,$5) RETURNING id`,[ws,title,user,agent,idx+1]);
  issueIds.push(id);
  await db.query(`INSERT INTO chat_issue_link (workspace_id,chat_session_id,issue_id,source) VALUES ($1,$2,$3,'manual')`,[ws,session,id]);
  await db.query(`INSERT INTO agent_task_queue (agent_id,runtime_id,issue_id,status,originator_user_id,accountable_user_id,originator_source,started_at,completed_at) VALUES ($1,$2,$3,$4,$5,$5,'member',now(),CASE WHEN $4='completed' THEN now() ELSE NULL END)`,[agent,runtime,id,idx===0?'queued':idx===1?'completed':'waiting_local_directory',user]);
 }
 await db.query(`INSERT INTO inbox_item (workspace_id,recipient_type,recipient_id,type,severity,title,body,issue_id,details) VALUES ($1,'member',$2,'workflow_owner_decision','action_required','Choose the compact preview','Which preview should we build?',$3,$4)`,[ws,user,issueIds[1],JSON.stringify({question_scope:'ordinary_task',intake_state:'pending',question:'Which preview should we build?',recommendation:'Use the compact card; expand media on click.',consequence:'The next step uses your selected layout.',options:'Compact card\nLarge preview', source_task_id:(await db.query(`SELECT id FROM agent_task_queue WHERE issue_id=$1`,[issueIds[1]])).rows[0].id,owner_relevant:'true'})]);
 await db.query(`INSERT INTO chat_message (chat_session_id,role,content,message_kind,created_at) VALUES ($1,'user','Keep this work together so I can manage it in one conversation.','message',now()-interval '1 minute'),($1,'assistant','This is an isolated review workspace with three fixture issues. Open Work to inspect the queued audit, the preview question, and the layout waiting for local access. No model worker is connected here, so sending a message queues it without generating an answer.','message',now())`,[session]);
 await db.query(`UPDATE workspace SET issue_counter=(SELECT max(number) FROM issue WHERE workspace_id=$1) WHERE id=$1`,[ws]);
 await db.query('COMMIT');
 const ids={workspace:ws,user,agent,runtime,session,issues:issueIds,url:`http://127.0.0.1:3221/mika-local-review/chat?session=${session}`};
 fs.writeFileSync(process.env.MIKA_REVIEW_FIXTURE_FILE,JSON.stringify(ids,null,2)); console.log(JSON.stringify(ids));
} catch(e) { await db.query('ROLLBACK'); throw e; } finally { await db.end(); }
