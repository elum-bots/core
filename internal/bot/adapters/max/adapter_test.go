package max

import (
	"testing"
	"time"

	"github.com/elum-bots/core/internal/bot"
	"github.com/elum-bots/core/internal/max-bot-api-client-go/schemes"
)

func TestPreferUserRoute(t *testing.T) {
	tests := []struct {
		name   string
		target recipientIDs
		want   bool
	}{
		{
			name:   "direct user dialog",
			target: recipientIDs{chat: 155232128, user: 155232128},
			want:   true,
		},
		{
			name:   "missing chat id falls back to user",
			target: recipientIDs{chat: 0, user: 155232128},
			want:   true,
		},
		{
			name:   "negative max dialog chat uses chat route",
			target: recipientIDs{chat: -72475136434816, user: 155232128},
			want:   false,
		},
		{
			name:   "chat with dedicated recipient user keeps chat route",
			target: recipientIDs{chat: -72475136434816, user: 155232128, chatUser: 999001},
			want:   false,
		},
		{
			name:   "dialog callback prefers sender user route",
			target: recipientIDs{chat: 262079376, user: 63378423, chatUser: 63378423, preferUser: true},
			want:   true,
		},
		{
			name:   "group or channel style positive chat uses chat route",
			target: recipientIDs{chat: 987654321, user: 155232128},
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := preferUserRoute(tc.target); got != tc.want {
				t.Fatalf("preferUserRoute(%+v) = %t, want %t", tc.target, got, tc.want)
			}
		})
	}
}

func TestEnrichRecipientsFromCurrentUpdate(t *testing.T) {
	upd := bot.Update{
		Platform: "max",
		ChatID:   "-72475136434816",
		UserID:   "155232128",
		Raw: &schemes.MessageCreatedUpdate{
			Message: schemes.Message{
				Sender: schemes.User{UserId: 155232128},
				Recipient: schemes.Recipient{
					ChatId:   -72475136434816,
					ChatType: schemes.ChatType("dialog"),
					UserId:   778899,
				},
			},
		},
	}

	got := enrichRecipientsFromCurrentUpdate(-72475136434816, upd, recipientIDs{
		chat: -72475136434816,
		user: 155232128,
	})

	want := recipientIDs{
		chat:       -72475136434816,
		user:       155232128,
		chatUser:   778899,
		preferUser: true,
	}
	if got != want {
		t.Fatalf("enrichRecipientsFromCurrentUpdate() = %+v, want %+v", got, want)
	}
}

func TestCandidateRoutes(t *testing.T) {
	target := recipientIDs{
		chat:       -72475136434816,
		user:       155232128,
		chatUser:   778899,
		preferUser: true,
	}

	got := candidateRoutes(target)
	want := []recipientRoute{
		{name: "user", user: 155232128},
		{name: "chat+user", chat: -72475136434816, user: 778899},
		{name: "chat+sender", chat: -72475136434816, user: 155232128},
		{name: "chat", chat: -72475136434816},
	}
	if len(got) != len(want) {
		t.Fatalf("candidateRoutes() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidateRoutes()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCurrentCallbackID(t *testing.T) {
	update := bot.Update{
		Platform: "max",
		Raw: &schemes.MessageCallbackUpdate{
			Callback: schemes.Callback{CallbackID: "cb-123"},
		},
	}

	got, ok := callbackIDFromUpdate(update)
	if !ok {
		t.Fatalf("callbackIDFromUpdate() ok = false, want true")
	}
	if got != "cb-123" {
		t.Fatalf("callbackIDFromUpdate() = %q, want %q", got, "cb-123")
	}
}

func TestWithHTTPTimeout(t *testing.T) {
	a, err := NewAdapter("test-token")
	if err != nil {
		t.Fatalf("NewAdapter() error = %v", err)
	}

	WithHTTPTimeout(95 * time.Second)(a)

	if a.api == nil {
		t.Fatalf("adapter api is nil")
	}
	if got := a.httpTimeout; got != 95*time.Second {
		t.Fatalf("http timeout = %v, want %v", got, 95*time.Second)
	}
}

func TestNormalizeUpdateSkipsNonDialogMessages(t *testing.T) {
	if _, ok := normalizeUpdate(&schemes.MessageCreatedUpdate{
		Message: schemes.Message{
			Sender: schemes.User{UserId: 155232128},
			Recipient: schemes.Recipient{
				ChatId:   987654321,
				ChatType: schemes.CHAT,
			},
			Body: schemes.MessageBody{Text: "hello"},
		},
	}); ok {
		t.Fatalf("expected chat message to be skipped")
	}

	if _, ok := normalizeUpdate(&schemes.MessageCallbackUpdate{
		Callback: schemes.Callback{
			CallbackID: "cb-1",
			Payload:    "action",
			User:       schemes.User{UserId: 155232128},
		},
		Message: &schemes.Message{
			Recipient: schemes.Recipient{
				ChatId:   987654321,
				ChatType: schemes.CHANNEL,
			},
		},
	}); ok {
		t.Fatalf("expected channel callback to be skipped")
	}
}
