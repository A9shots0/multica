// Bootstrap only the disposable acceptance worker; never reuse a live profile.
import pg from "pg";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import crypto from "node:crypto";

const url = process.env.DATABASE_URL;
if (!url || new URL(url).hostname !== "127.0.0.1" || new URL(url).port !== "55438" || new URL(url).pathname !== "/multica_review") throw Error("Use the isolated mika review database");
const profile = "mika-chat-review-20260924";
const dir = path.join(os.homedir(), ".multica", "profiles", profile);
if (fs.existsSync(path.join(dir, "config.json"))) throw Error("Review profile already exists; preserve it");
const db = new pg.Client({connectionString:url});
await db.connect();
try {
  const owner = (await db.query(`SELECT u.id,w.id AS workspace_id FROM "user" u JOIN member m ON m.user_id=u.id JOIN workspace w ON w.id=m.workspace_id WHERE u.email='mika-review@example.test' AND w.slug='mika-local-review'`)).rows[0];
  if (!owner) throw Error("Seed the disposable workspace first");
  const token = `mul_${crypto.randomBytes(20).toString("hex")}`;
  await db.query(`INSERT INTO personal_access_token(user_id,name,token_hash,token_prefix,expires_at) VALUES($1,'Disposable Mika local trial',$2,$3,now()+interval '1 day')`,[owner.id,crypto.createHash("sha256").update(token).digest("hex"),token.slice(0,12)]);
  fs.mkdirSync(dir,{recursive:true});
  fs.writeFileSync(path.join(dir,"config.json"),JSON.stringify({server_url:"http://127.0.0.1:3220",app_url:"http://127.0.0.1:3221",workspace_id:owner.workspace_id,token,device_name:"Mika isolated local trial",runtime_name:"Mika local trial",workspaces_root:"C:/Dev/_work/mika-chat-worker-20260924",max_concurrent_tasks:1,agent_timeout:"5m",disable_auto_update:true,disable_auto_reload:true},null,2),{mode:0o600});
  console.log(`Prepared isolated profile ${profile}; token material omitted.`);
} finally { await db.end(); }
