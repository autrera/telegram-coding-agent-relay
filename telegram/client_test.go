package telegram

import (
	"encoding/json"
	"testing"
)

func TestSetMyCommandsPayloadMarshaling(t *testing.T) {
	cmds := []BotCommand{
		{Command: "new", Description: "Start a brand new conversation"},
		{Command: "help", Description: "Show help and command menu"},
	}
	payload := map[string]any{"commands": cmds}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var got struct {
		Commands []BotCommand `json:"commands"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(got.Commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(got.Commands))
	}
	if got.Commands[0].Command != "new" || got.Commands[0].Description != "Start a brand new conversation" {
		t.Errorf("unexpected first command: %+v", got.Commands[0])
	}
	if got.Commands[1].Command != "help" || got.Commands[1].Description != "Show help and command menu" {
		t.Errorf("unexpected second command: %+v", got.Commands[1])
	}
}

func TestAPIResponseBoolParsing(t *testing.T) {
	var res APIResponse[bool]
	if err := json.Unmarshal([]byte(`{"ok":true,"result":true}`), &res); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !res.OK || !res.Result {
		t.Errorf("unexpected response: %+v", res)
	}

	var errRes APIResponse[bool]
	if err := json.Unmarshal([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: command list is empty"}`), &errRes); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if errRes.OK || errRes.ErrorCode != 400 || errRes.Description == "" {
		t.Errorf("unexpected error response: %+v", errRes)
	}
}
