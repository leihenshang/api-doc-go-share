package syncapi

// 线协议守卫：这里把「网络上真正出现的 JSON 键」钉死。
//
// 为什么值得单独测：两端现在都 alias 同一个结构，Go 侧字段名改动不会编译失败，
// 但 `json` tag 一改就会静默改变线上格式（老客户端解析不到 → 字段丢失、不报错）。
// 所以这里的断言是**按 JSON 键**写的，改 tag 必须先改本测试（等于一次有意识的协议变更）。

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// keysOf 返回序列化后的顶层键集合（排序后）。
func keysOf(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func assertKeys(t *testing.T, what string, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("%s 的线格式键变了：\n got=%v\nwant=%v\n（改 tag 等于改协议，需同步升 Proto 与两端 require）",
			what, got, want)
	}
}

func TestWireKeys(t *testing.T) {
	// 带 omitempty 的字段在零值时不出现在 JSON 里，所以这里逐类型给齐值。
	enabled := true
	assertKeys(t, "Item", keysOf(t, Item{
		Type: TypeAPI, UID: "u", Op: OpUpsert, Payload: json.RawMessage(`{}`),
		ItemRev: 3, Deleted: true,
	}), []string{"type", "uid", "op", "payload", "item_rev", "deleted"})

	assertKeys(t, "Changes", keysOf(t, Changes{Cursor: 1, Items: []Item{}, HasMore: true, Gap: true}),
		[]string{"cursor", "items", "has_more", "gap"})

	assertKeys(t, "Snapshot", keysOf(t, Snapshot{Cursor: 1, Items: []Item{}, HasMore: true, Offset: 2}),
		[]string{"cursor", "items", "has_more", "offset"})

	assertKeys(t, "State", keysOf(t, State{Cursor: 1, OldestAvailable: 2}),
		[]string{"cursor", "oldest_available"})

	assertKeys(t, "PushReq", keysOf(t, PushReq{ProjectID: 1, DeviceID: "d", DeviceName: "n", Ops: []Op{}}),
		[]string{"project_id", "device_id", "device_name", "ops"})

	assertKeys(t, "Op", keysOf(t, Op{OpID: "o", Op: OpDelete, Type: TypeDoc, UID: "u", BaseRev: 4, Payload: json.RawMessage(`{}`)}),
		[]string{"op_id", "op", "type", "uid", "base_rev", "payload"})

	assertKeys(t, "OpResult", keysOf(t, OpResult{
		OpID: "o", Status: StatusConflict, ItemRev: 5, Reason: "r", Server: json.RawMessage(`{}`),
	}), []string{"op_id", "status", "item_rev", "reason", "server"})

	assertKeys(t, "PushResp", keysOf(t, PushResp{Results: []OpResult{}, Cursor: 9}),
		[]string{"results", "cursor"})

	assertKeys(t, "Meta", keysOf(t, Meta{
		ServerVersion: "1", APIVersion: "v1", SyncProto: Proto, Features: []string{"sync"},
		MinClientVersion: "0.1",
	}), []string{"server_version", "api_version", "sync_proto", "features", "min_client_version"})

	// Payload：一条 group 变更只应带 title/parent_uid/priority，不该把 api/doc 的字段也带上。
	assertKeys(t, "Payload(group)", keysOf(t, Payload{Title: "t", PID: "p", Priority: 1}),
		[]string{"title", "parent_uid", "priority"})
	assertKeys(t, "Payload(variable)", keysOf(t, Payload{
		Name: "n", Value: "v", Secret: true, Enabled: &enabled, EnvUID: "e",
	}), []string{"name", "value", "secret", "enabled", "env_uid"})
	assertKeys(t, "Payload(example)", keysOf(t, Payload{
		Name: "n", APIUID: "a", Sample: "{}", Status: 200, ContentType: "application/json", Seq: 1,
	}), []string{"name", "api_uid", "sample", "status", "content_type", "seq"})
}

// TestConstants 常量值即线上字面量，改动会让老客户端认不出类型。
func TestConstants(t *testing.T) {
	want := map[string]string{
		"group": TypeGroup, "api": TypeAPI, "doc": TypeDoc,
		"example": TypeExample, "variable": TypeVariable, "variable_env": TypeVarEnv,
		"upsert": OpUpsert, "delete": OpDelete,
		"ok": StatusOK, "conflict": StatusConflict, "rejected": StatusRejected,
	}
	for k, v := range want {
		if k != v {
			t.Fatalf("常量值应为 %q，got %q", k, v)
		}
	}
	if Proto != 1 {
		t.Fatalf("Proto 改动属于协议升级，需同步两端，got %d", Proto)
	}
}
