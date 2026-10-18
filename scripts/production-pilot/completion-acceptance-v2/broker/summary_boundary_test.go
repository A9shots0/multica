package main

import (
 "encoding/json"
 "os"
 "strings"
 "testing"
)

func TestSummaryUTF8Boundary(t *testing.T) {
 for _, tc := range []struct{name,summary string; accepted bool}{
  {"ascii600",strings.Repeat("x",600),true},
  {"ascii601",strings.Repeat("x",601),false},
  {"unicode600",strings.Repeat("é",300),true},
  {"unicode602",strings.Repeat("é",301),false},
 } {
  t.Run(tc.name,func(t *testing.T){
   raw,_:=json.Marshal(map[string]any{"summary":tc.summary,"verdict":"pass","findings":[]string{}})
   for _,kind:=range []string{"implementation","review"} {
    _,err:=parseResult(string(raw),kind)
    if (err==nil)!=tc.accepted { t.Fatalf("%s: accepted=%v; wanted=%v",kind,err==nil,tc.accepted) }
   }
  })
 }
}

func TestPreservedLiveOversizeResultRemainsRejected(t *testing.T) {
 raw,err:=os.ReadFile("testdata/oversized-publisher-result.txt"); if err!=nil{t.Fatal(err)}
 if _,err:=parseResult(string(raw),"implementation"); err==nil {t.Fatal("the failed live response must not become an accepted result")}
}
