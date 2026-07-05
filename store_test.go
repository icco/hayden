package hayden

import (
	"context"
	"testing"
	"time"
)

func TestStoreCRUD(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()

	tg := &Target{Name: "t1", URL: "https://example.com", MatchType: "substring", MatchValue: "x", FetchMode: "http", NotifyMode: "once", Enabled: true}
	if err := s.Create(ctx, tg); err != nil {
		t.Fatalf("create: %v", err)
	}
	if tg.ID == 0 {
		t.Fatal("expected ID to be set after Create")
	}

	got, err := s.Get(ctx, tg.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "t1" || got.URL != "https://example.com" {
		t.Errorf("unexpected target: %+v", got)
	}

	dis := &Target{Name: "d", URL: "https://e2.com", MatchType: "substring", Enabled: false}
	if err := s.Create(ctx, dis); err != nil {
		t.Fatalf("create disabled: %v", err)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2", len(list))
	}

	en, err := s.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(en) != 1 || en[0].ID != tg.ID {
		t.Fatalf("ListEnabled = %+v, want only %d", en, tg.ID)
	}

	now := time.Now().UTC().Truncate(time.Second)
	got.LastStatus = "ok"
	got.LastMatched = true
	got.LastRunAt = &now
	got.LastMatchAt = &now
	if err := s.SaveRunState(ctx, got); err != nil {
		t.Fatalf("save run-state: %v", err)
	}
	reload, err := s.Get(ctx, got.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reload.LastStatus != "ok" || !reload.LastMatched || reload.LastRunAt == nil {
		t.Errorf("run-state not persisted: %+v", reload)
	}

	cnt, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 2 {
		t.Errorf("Count = %d, want 2", cnt)
	}

	if err := s.Delete(ctx, dis.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = s.List(ctx)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List after delete = %d, want 1", len(list))
	}
}
