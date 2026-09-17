package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/institucional/symphonia/backend/internal/call"
	"github.com/institucional/symphonia/backend/internal/translation"
)

func translationTestRoom(t *testing.T, opener translation.SessionOpener) (*Hub, *httptest.Server) {
	t.Helper()
	t.Setenv("JWT_SECRET", "websocket-test-secret")
	now := time.Now().UTC()
	store := &fakeCallStore{value: &call.Call{
		ID: 10, Code: "room1234", HostUserID: 1, Status: call.StatusActive, CreatedAt: now, UpdatedAt: now,
		Participants: []call.Participant{
			{UserID:1, Name:"Ana", SpokenLanguage:call.LanguagePortuguese, HeardLanguage:call.LanguagePortuguese, JoinedAt:now},
			{UserID:2, Name:"Ben", SpokenLanguage:call.LanguageEnglish, HeardLanguage:call.LanguageEnglish, JoinedAt:now},
		},
	}}
	hub := NewHub()
	mux := http.NewServeMux()
	mux.Handle("GET /api/calls/{code}/ws", NewHandler(store, hub, opener))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	t.Cleanup(hub.Close)
	return hub, server
}

func TestPortugueseEnglishBothDirectionsAndShutdown(t *testing.T) {
	hub, server := translationTestRoom(t, translation.NewMockService())
	a := connectAndJoin(t, server.URL, 1, "a")
	defer a.CloseNow()
	b := connectAndJoin(t, server.URL, 2, "b")
	defer b.CloseNow()
	_ = readEnvelope(t, a)
	for _, direction := range []struct{ reverse bool; target translation.Language }{{false,translation.LanguageEnglish},{true,translation.LanguagePortuguese}} {
		sender, receiver := a,b
		if direction.reverse { sender,receiver=b,a }
		writeBinary(t,sender,testAudioFrame(99,0,0))
		_ = readEnvelope(t,receiver)
		output := readEnvelope(t,receiver)
		var data transcriptionEvent
		decodeEventData(t,output,&data)
		if output.Type!=EventOutputTranscription || data.Language!=direction.target || data.Text=="" { t.Fatalf("incorrect translation: %+v",data) }
		_ = readTranslatedFrame(t,receiver)
		_ = readTranslatedFrame(t,receiver)
		_ = readEnvelope(t,sender)
		_ = readEnvelope(t,sender)
	}
	hub.mu.Lock()
	count := len(hub.rooms["room1234"].sessions)
	hub.mu.Unlock()
	if count!=2 { t.Fatalf("sessions=%d, want independent sessions in both directions",count) }
	hub.Close()
	ctx,cancel:=context.WithTimeout(context.Background(),time.Second)
	defer cancel()
	if _,_,err:=a.Read(ctx); err==nil { t.Fatal("shutdown left client socket open") }
}

type blockedOpener struct { started chan struct{}; stopped chan struct{} }
func (b *blockedOpener) OpenLiveSession(ctx context.Context, _ translation.OpenRequest) (translation.LiveSession,error) {
	close(b.started)
	<-ctx.Done()
	close(b.stopped)
	return nil,ctx.Err()
}

func TestSlowProviderDoesNotBlockMuteAndDisconnectCancelsSetup(t *testing.T) {
	opener:=&blockedOpener{started:make(chan struct{}),stopped:make(chan struct{})}
	_,server:=translationTestRoom(t,opener)
	a:=connectAndJoin(t,server.URL,1,"a")
	defer a.CloseNow()
	b:=connectAndJoin(t,server.URL,2,"b")
	defer b.CloseNow()
	_ = readEnvelope(t,a)
	writeBinary(t,a,testAudioFrame(99,0,0))
	select { case <-opener.started: case <-time.After(time.Second): t.Fatal("provider not opened") }
	ctx,cancel:=context.WithTimeout(context.Background(),time.Second)
	defer cancel()
	if err:=a.Write(ctx,1,[]byte(`{"version":1,"type":"mute_state","request_id":"mute","data":{"muted":true}}`)); err!=nil { t.Fatal(err) }
	if event:=readEnvelope(t,a); event.Type!=EventMuteState { t.Fatalf("mute blocked: %s",event.Type) }
	_ = a.CloseNow()
	select { case <-opener.stopped: case <-time.After(2*time.Second): t.Fatal("provider setup was not canceled") }
}
