package max

import (
	"strings"
	"unsafe"

	maxbot "github.com/elum-bots/core/internal/max-bot-api-client-go"
	"github.com/elum-bots/core/internal/max-bot-api-client-go/schemes"
)

type messageLayout struct {
	userID  int64
	chatID  int64
	reset   bool
	message *schemes.NewMessageBody
}

func AttachPhotoReference(msg *maxbot.Message, ref string) bool {
	ref = strings.TrimSpace(ref)
	if msg == nil || ref == "" {
		return false
	}
	body := extractMessageBody(msg)
	if body == nil {
		return false
	}
	payload := schemes.PhotoAttachmentRequestPayload{}
	if strings.Contains(ref, "://") {
		payload.Url = ref
	} else {
		payload.Token = ref
	}
	body.Attachments = append(body.Attachments, schemes.NewPhotoAttachmentRequest(payload))
	return true
}

func extractMessageBody(msg *maxbot.Message) *schemes.NewMessageBody {
	if msg == nil {
		return nil
	}
	layout := (*messageLayout)(unsafe.Pointer(msg))
	return layout.message
}
