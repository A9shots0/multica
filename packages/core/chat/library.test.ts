import { describe, expect, it } from "vitest";
import { readLibrary, moveToGroup } from "./library";
import { createChatStore } from "./store";
describe("conversation organization", () => {
 it("moves only membership, preserving other groups and conversations", () => {
  const library = readLibrary(JSON.stringify({groups:[{id:"a",name:"Projects to discuss",sessionIds:["one","two"]},{id:"b",name:"This week",sessionIds:["three"]}]}));
  expect(moveToGroup(library,"one","b").groups.map(g=>g.sessionIds)).toEqual([["two"],["three","one"]]);
 });
 it("rejects malformed persisted preferences",()=>{expect(readLibrary('{bad').groups).toEqual([]);expect(readLibrary('{"groups":7}').grouping).toBe("recent");});
 it("restores organization after reload without changing drafts",()=>{
  const data = new Map<string,string>();
  const storage = {getItem:(k:string)=>data.get(k)??null,setItem:(k:string,v:string)=>{data.set(k,v)},removeItem:(k:string)=>{data.delete(k)}};
  const first=createChatStore({storage}); first.getState().setInputDraft("one","Still writing");
  const library=readLibrary(JSON.stringify({grouping:"custom",filter:"needs_you",groups:[{id:"week",name:"This week",sessionIds:["one"]}]}));
  first.getState().setLibrary(library);
  const restored=createChatStore({storage});expect(restored.getState().library).toEqual(library);expect(restored.getState().inputDrafts.one).toBe("Still writing");
 });
});
