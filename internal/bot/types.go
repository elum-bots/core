package bot

import (
	"context"
	"time"
)

type MessageKind string

const (
	MessageKindText  MessageKind = "text"
	MessageKindPhoto MessageKind = "photo"
	MessageKindVideo MessageKind = "video"
	MessageKindAudio MessageKind = "audio"
	MessageKindVoice MessageKind = "voice"
	MessageKindFile  MessageKind = "file"
	MessageKindOther MessageKind = "other"
)

type Attachment struct {
	ID   string
	Name string
	MIME string
	URL  string
	Size int64
}

type Message struct {
	Text        string
	Kind        MessageKind
	Attachments []Attachment
}

type Update struct {
	Platform  string
	ChatID    string
	UserID    string
	MessageID string
	Event     string
	Command   string
	Args      []string
	ButtonID  string
	Message   Message
	Payload   map[string]any
	Raw       any
	Time      time.Time
}

type Input struct {
	Text        string
	ButtonID    string
	Kind        MessageKind
	Attachments []Attachment
	Payload     map[string]any
	Raw         any
	Update      Update
}

type ButtonKind string

const (
	ButtonKindCallback ButtonKind = "callback"
	ButtonKindURL      ButtonKind = "url"
)

type Button struct {
	ID   string
	Text string
	Kind ButtonKind
	URL  string
}

type ButtonRow []Button

func Row(btns ...Button) ButtonRow {
	return ButtonRow(btns)
}

func Btn(id, text string) Button {
	return Button{ID: id, Text: text, Kind: ButtonKindCallback}
}

func URLBtn(text, rawURL string) Button {
	return Button{Text: text, Kind: ButtonKindURL, URL: rawURL}
}

type OutMessage struct {
	Text    string
	Buttons []ButtonRow
	Media   []OutMedia
}

type SendOption func(*OutMessage)

func WithButtons(rows ...ButtonRow) SendOption {
	return func(m *OutMessage) {
		m.Buttons = rows
	}
}

type OutMediaKind string

const (
	OutMediaImage OutMediaKind = "image"
	OutMediaVideo OutMediaKind = "video"
	OutMediaAudio OutMediaKind = "audio"
	OutMediaFile  OutMediaKind = "file"
)

type OutMedia struct {
	Kind    OutMediaKind
	Path    string
	Name    string
	Bytes   []byte
	Caption string
}

func WithMedia(media ...OutMedia) SendOption {
	return func(m *OutMessage) {
		m.Media = append(m.Media, media...)
	}
}

func Image(path string) OutMedia {
	return OutMedia{Kind: OutMediaImage, Path: path}
}

func ImageBytes(name string, raw []byte) OutMedia {
	return OutMedia{Kind: OutMediaImage, Name: name, Bytes: append([]byte(nil), raw...)}
}

func Video(path string) OutMedia {
	return OutMedia{Kind: OutMediaVideo, Path: path}
}

func VideoBytes(name string, raw []byte) OutMedia {
	return OutMedia{Kind: OutMediaVideo, Name: name, Bytes: append([]byte(nil), raw...)}
}

func Audio(path string) OutMedia {
	return OutMedia{Kind: OutMediaAudio, Path: path}
}

func AudioBytes(name string, raw []byte) OutMedia {
	return OutMedia{Kind: OutMediaAudio, Name: name, Bytes: append([]byte(nil), raw...)}
}

func File(path string) OutMedia {
	return OutMedia{Kind: OutMediaFile, Path: path}
}

func FileBytes(name string, raw []byte) OutMedia {
	return OutMedia{Kind: OutMediaFile, Name: name, Bytes: append([]byte(nil), raw...)}
}

type Adapter interface {
	Name() string
	Start(ctx context.Context, dispatch Dispatcher) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, chatID string, msg OutMessage) error
}

type Dispatcher interface {
	Dispatch(ctx context.Context, update Update) error
}

type EventHandler func(ctx context.Context, args ...string) error
type Middleware func(next EventHandler) EventHandler
