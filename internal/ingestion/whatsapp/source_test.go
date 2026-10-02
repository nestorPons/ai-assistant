package whatsapp

import (
	"testing"

	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

func strp(s string) *string { return &s }

func TestClientIdentifier(t *testing.T) {
	cases := []struct {
		name string
		info types.MessageInfo
		want string
	}{
		{
			name: "chat 1:1",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Sender: types.JID{User: "34612345678", Server: types.DefaultUserServer},
					Chat:   types.JID{User: "34612345678", Server: types.DefaultUserServer},
				},
			},
			want: "+34612345678",
		},
		{
			name: "grupo",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Sender:  types.JID{User: "34612345678", Server: types.DefaultUserServer},
					Chat:    types.JID{User: "123456789-123456", Server: types.GroupServer},
					IsGroup: true,
				},
			},
			want: "123456789-123456@g.us",
		},
		{
			name: "chat 1:1 por LID con telefono en SenderAlt",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Sender:    types.JID{User: "12345678901234", Server: types.HiddenUserServer},
					SenderAlt: types.JID{User: "34612345678", Server: types.DefaultUserServer},
					Chat:      types.JID{User: "12345678901234", Server: types.HiddenUserServer},
				},
			},
			want: "+34612345678",
		},
		{
			name: "chat 1:1 solo LID sin telefono",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Sender: types.JID{User: "12345678901234", Server: types.HiddenUserServer},
					Chat:   types.JID{User: "12345678901234", Server: types.HiddenUserServer},
				},
			},
			want: "12345678901234@lid",
		},
		{
			name: "sin usuario",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Chat: types.JID{User: "34612345678", Server: types.DefaultUserServer},
				},
			},
			want: "",
		},
	}
	for _, c := range cases {
		if got := clientIdentifier(c.info); got != c.want {
			t.Errorf("%s: clientIdentifier = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestIsDirectChatServer(t *testing.T) {
	direct := []string{types.DefaultUserServer, types.HiddenUserServer, types.LegacyUserServer}
	for _, srv := range direct {
		if !isDirectChatServer(srv) {
			t.Errorf("isDirectChatServer(%q) = false, want true", srv)
		}
	}
	for _, srv := range []string{types.GroupServer, types.BroadcastServer, types.NewsletterServer, "status"} {
		if isDirectChatServer(srv) {
			t.Errorf("isDirectChatServer(%q) = true, want false", srv)
		}
	}
}

func TestMessageText(t *testing.T) {
	cases := []struct {
		name string
		m    *waE2E.Message
		want string
	}{
		{"conversacion", &waE2E.Message{Conversation: strp("hola")}, "hola"},
		{"texto extendido", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: strp("necesito x")}}, "necesito x"},
		{"caption imagen", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: strp("mira esto")}}, "mira esto"},
		{"caption video", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{Caption: strp("clip")}}, "clip"},
		{"caption documento", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{Caption: strp("doc")}}, "doc"},
		{"vacio", &waE2E.Message{}, ""},
		{"nil", nil, ""},
	}
	for _, c := range cases {
		if got := messageText(c.m); got != c.want {
			t.Errorf("%s: messageText = %q, want %q", c.name, got, c.want)
		}
	}
}
