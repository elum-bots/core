package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type StepHandler func(dc DialogContext) error
type BranchHandler func(dc DialogContext) (nextStepID string, err error)
type EndHandler func(dc DialogContext) error
type MapFunc func(values map[string]any) map[string]any
type ClickHandler func(dc DialogContext) *Dialog

type DialogContext interface {
	Context() context.Context
	Update() Update

	Get(key string) any
	Set(key string, val any)
	Value(key string) any
	Input(key string) Input
	MustString(key string) string
	MustInt(key string) int

	Reply(text string, opts ...SendOption) error
	Next(stepID string) error
	Go(stepID string) error
	Finish(result map[string]any) error
}

type AskOption func(*askConfig)
type Validator func(in Input) error

type askConfig struct {
	validator Validator
}

func WithValidator(v Validator) AskOption {
	return func(cfg *askConfig) {
		cfg.validator = v
	}
}

func IntRange(min, max int) Validator {
	return func(in Input) error {
		n, err := strconv.Atoi(in.Text)
		if err != nil {
			return ErrInvalidInput
		}
		if n < min || n > max {
			return ErrInvalidInput
		}
		return nil
	}
}

type dialogStepKind string

const (
	stepKindAsk    dialogStepKind = "ask"
	stepKindSend   dialogStepKind = "send"
	stepKindStep   dialogStepKind = "step"
	stepKindBranch dialogStepKind = "branch"
	stepKindSub    dialogStepKind = "subdialog"
	stepKindEnd    dialogStepKind = "end"
)

type dialogStep struct {
	kind dialogStepKind
	id   string

	prompt string
	askCfg askConfig
	opts   []SendOption

	stepHandler   StepHandler
	branchHandler BranchHandler
	clickHandlers map[string]ClickHandler

	subDialog string
	mapIn     MapFunc
	mapOut    MapFunc

	endHandler EndHandler
}

type DialogDef struct {
	name  string
	steps []dialogStep
	idx   map[string]int
	seed  map[string]any

	onFinish EndHandler
}

type Dialog struct {
	def      *DialogDef
	lastStep string
}

func NewDialog() *Dialog {
	return &Dialog{
		def: &DialogDef{
			idx:  make(map[string]int),
			seed: make(map[string]any),
		},
	}
}

func (d *Dialog) Ask(id string, prompt string, opts ...AskOption) *Dialog {
	cfg := askConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	d.add(dialogStep{kind: stepKindAsk, id: id, prompt: prompt, askCfg: cfg})
	return d
}

func (d *Dialog) Send(text string, opts ...SendOption) *Dialog {
	id := fmt.Sprintf("send_%d", len(d.def.steps))
	d.add(dialogStep{
		kind:          stepKindSend,
		id:            id,
		prompt:        text,
		opts:          append([]SendOption(nil), opts...),
		clickHandlers: make(map[string]ClickHandler),
	})
	d.lastStep = id
	return d
}

func (d *Dialog) OnClick(buttonID string, h ClickHandler) *Dialog {
	if h == nil || d.lastStep == "" {
		return d
	}
	idx, ok := d.def.idx[d.lastStep]
	if !ok {
		return d
	}
	if d.def.steps[idx].clickHandlers == nil {
		d.def.steps[idx].clickHandlers = make(map[string]ClickHandler)
	}
	d.def.steps[idx].clickHandlers[buttonID] = h
	return d
}

func (d *Dialog) WithValue(key string, val any) *Dialog {
	if d.def.seed == nil {
		d.def.seed = make(map[string]any)
	}
	d.def.seed[key] = val
	return d
}

func (d *Dialog) OnFinish(h EndHandler) *Dialog {
	d.def.onFinish = h
	return d
}

func (d *Dialog) add(step dialogStep) {
	if d.def.idx == nil {
		d.def.idx = make(map[string]int)
	}
	if step.id == "" {
		step.id = fmt.Sprintf("step_%d", len(d.def.steps))
	}
	d.def.idx[step.id] = len(d.def.steps)
	d.def.steps = append(d.def.steps, step)
}

type dialogSession struct {
	Frames []dialogFrame
}

type dialogFrame struct {
	Dialog          string
	RuntimeDef      *DialogDef
	Index           int
	WaitingStepID   string
	Values          map[string]any
	ParentSubStepID string
	BranchAnchor    int
	BranchChoice    int
	BranchPending   bool
}

type dialogContext struct {
	ctx    context.Context
	bot    *Bot
	update Update
	frame  *dialogFrame

	nextStepID string
	finishNow  bool
	finishData map[string]any
}

func (d *dialogContext) Context() context.Context { return d.ctx }
func (d *dialogContext) Update() Update           { return d.update }
func (d *dialogContext) Get(key string) any       { return d.frame.Values[key] }
func (d *dialogContext) Set(key string, val any)  { d.frame.Values[key] = val }
func (d *dialogContext) Value(key string) any     { return d.frame.Values[key] }

func (d *dialogContext) Input(key string) Input {
	v := d.frame.Values[key]
	in, _ := v.(Input)
	return in
}

func (d *dialogContext) MustString(key string) string {
	v, _ := d.frame.Values[key].(string)
	return v
}

func (d *dialogContext) MustInt(key string) int {
	s, ok := d.frame.Values[key].(string)
	if !ok {
		if i, ok := d.frame.Values[key].(int); ok {
			return i
		}
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func (d *dialogContext) Reply(text string, opts ...SendOption) error {
	return d.bot.Send(d.ctx, d.update.ChatID, text, opts...)
}

func (d *dialogContext) Next(stepID string) error {
	d.nextStepID = stepID
	return nil
}

func (d *dialogContext) Go(stepID string) error {
	d.nextStepID = stepID
	return nil
}

func (d *dialogContext) Finish(result map[string]any) error {
	d.finishNow = true
	d.finishData = result
	return nil
}

func inputFromUpdate(update Update) Input {
	atts := append([]Attachment(nil), update.Message.Attachments...)
	payload := map[string]any{}
	for k, v := range update.Payload {
		payload[k] = v
	}
	return Input{
		Text:        update.Message.Text,
		ButtonID:    update.ButtonID,
		Kind:        update.Message.Kind,
		Attachments: atts,
		Payload:     payload,
		Raw:         update.Raw,
		Update:      update,
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneDialogDef(def *DialogDef) *DialogDef {
	if def == nil {
		return &DialogDef{idx: map[string]int{}, seed: map[string]any{}}
	}
	c := *def
	c.steps = append([]dialogStep(nil), def.steps...)
	c.idx = make(map[string]int, len(def.idx))
	for k, v := range def.idx {
		c.idx[k] = v
	}
	c.seed = cloneMap(def.seed)
	return &c
}

func (b *Bot) Dialog(name string, spec ...any) *Dialog {
	if len(spec) == 0 {
		def, ok := b.dialogByName(name)
		if !ok {
			d := NewDialog()
			d.def.name = name
			return d
		}
		cl := cloneDialogDef(def)
		cl.name = name
		return &Dialog{def: cl}
	}
	def := &DialogDef{name: name, idx: make(map[string]int), seed: make(map[string]any)}
	switch v := spec[0].(type) {
	case func(d *Dialog):
		v(&Dialog{def: def})
	case func() *Dialog:
		built := v()
		if built != nil && built.def != nil {
			def = cloneDialogDef(built.def)
			def.name = name
		}
	default:
		return &Dialog{def: def}
	}
	b.mu.Lock()
	b.dialogs[name] = def
	b.mu.Unlock()
	return &Dialog{def: cloneDialogDef(def)}
}

func (b *Bot) StartDialog(ctx context.Context, name string, input map[string]any) error {
	b.mu.RLock()
	_, ok := b.dialogs[name]
	b.mu.RUnlock()
	if !ok {
		return ErrDialogNotFound
	}
	rd, rok := runtimeFrom(ctx)
	if !rok {
		return ErrInvalidInput
	}
	vals := map[string]any{}
	for k, v := range input {
		vals[k] = v
	}
	if def, ok := b.dialogByName(name); ok {
		for k, v := range def.seed {
			if _, exists := vals[k]; !exists {
				vals[k] = v
			}
		}
	}
	s := &dialogSession{Frames: []dialogFrame{{Dialog: name, Values: vals}}}
	key := dialogSessionKey(rd.update)
	b.saveSession(key, s)
	return b.runDialog(ctx, rd.update, false)
}

func (b *Bot) hasDialogSession(_ context.Context, update Update) (bool, error) {
	key := dialogSessionKey(update)
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()
	_, ok := b.sessions[key]
	return ok, nil
}

func (b *Bot) processDialog(ctx context.Context, update Update) error {
	return b.runDialog(withRuntime(ctx, runtimeData{bot: b, update: update}), update, true)
}

func (b *Bot) runDialog(ctx context.Context, update Update, hasInput bool) error {
	key := dialogSessionKey(update)
	s, ok := b.loadSession(key)
	if !ok {
		return nil
	}
	if hasInput && isCloseCommand(update) {
		b.deleteSession(key)
		if h, ok := b.lookupHandler("close"); ok {
			rctx := withRuntime(ctx, runtimeData{bot: b, update: update})
			if err := h(rctx); err != nil {
				return err
			}
			return nil
		}
		return b.Send(ctx, update.ChatID, "Диалог закрыт")
	}
	if hasInput && isBackCommand(update) {
		if b.stepBack(s) {
			hasInput = false
		} else {
			b.saveSession(key, s)
			return b.Send(ctx, update.ChatID, "Нет предыдущего шага")
		}
	}
	if hasInput && isBlockedDialogCommand(update) {
		b.saveSession(key, s)
		return b.Send(ctx, update.ChatID, "Во время диалога доступны только /back и /close")
	}
	if hasInput && update.ButtonID != "" && !b.isExpectedButtonInput(s, update.ButtonID) {
		if h, ok := b.lookupHandler("button:" + update.ButtonID); ok {
			b.deleteSession(key)
			rctx := withRuntime(ctx, runtimeData{bot: b, update: update})
			return h(rctx)
		}
		b.saveSession(key, s)
		return nil
	}
	for {
		if len(s.Frames) == 0 {
			b.deleteSession(key)
			return nil
		}

		frame := &s.Frames[len(s.Frames)-1]
		def, ok := b.resolveFrameDef(frame)
		if !ok {
			b.deleteSession(key)
			return ErrDialogNotFound
		}
		if frame.Index >= len(def.steps) {
			if def.onFinish != nil {
				dc := &dialogContext{ctx: withRuntime(ctx, runtimeData{bot: b, update: update}), bot: b, update: update, frame: frame}
				if err := def.onFinish(dc); err != nil {
					return err
				}
			}
			b.popFrameToParent(s, frame)
			if len(s.Frames) == 0 {
				b.deleteSession(key)
				return nil
			}
			continue
		}

		step := def.steps[frame.Index]
		dc := &dialogContext{ctx: withRuntime(ctx, runtimeData{bot: b, update: update}), bot: b, update: update, frame: frame}

		switch step.kind {
		case stepKindSend:
			if frame.WaitingStepID == step.id && hasInput {
				if step.clickHandlers != nil {
					if h, ok := step.clickHandlers[update.ButtonID]; ok {
						next := h(dc)
						frame.WaitingStepID = ""
						if next != nil {
							if err := b.transitionDialogFrame(s, frame, next, dc); err != nil {
								return err
							}
							hasInput = false
							continue
						}
						frame.Index++
						hasInput = false
						continue
					}
				}
				b.saveSession(key, s)
				return nil
			}
			if err := dc.Reply(step.prompt, step.opts...); err != nil {
				return err
			}
			if len(step.clickHandlers) > 0 {
				frame.WaitingStepID = step.id
				b.saveSession(key, s)
				return nil
			}
			frame.Index++
			continue
		case stepKindAsk:
			if frame.WaitingStepID == step.id && hasInput {
				if step.clickHandlers != nil {
					if h, ok := step.clickHandlers[update.ButtonID]; ok {
						next := h(dc)
						frame.WaitingStepID = ""
						if next != nil {
							if err := b.transitionDialogFrame(s, frame, next, dc); err != nil {
								return err
							}
							hasInput = false
							continue
						}
						frame.Index++
						hasInput = false
						continue
					}
				}
				in := inputFromUpdate(update)
				if step.askCfg.validator != nil {
					if err := step.askCfg.validator(in); err != nil {
						if sendErr := dc.Reply("Некорректный ввод, попробуйте еще раз"); sendErr != nil {
							return sendErr
						}
						b.saveSession(key, s)
						return nil
					}
				}
				frame.Values[step.id] = in
				frame.WaitingStepID = ""
				if frame.BranchPending && frame.Index == frame.BranchChoice {
					frame.Index = findNextEndIndex(def, frame.BranchAnchor)
					frame.BranchPending = false
				} else {
					frame.Index++
				}
				hasInput = false
				continue
			}
			if strings.TrimSpace(step.prompt) != "" {
				if err := dc.Reply(step.prompt); err != nil {
					return err
				}
			}
			frame.WaitingStepID = step.id
			b.saveSession(key, s)
			return nil
		case stepKindStep:
			if step.stepHandler != nil {
				if err := step.stepHandler(dc); err != nil {
					return err
				}
			}
			if dc.finishNow {
				if dc.finishData != nil {
					for k, v := range dc.finishData {
						frame.Values[k] = v
					}
				}
				b.popFrameToParent(s, frame)
				if len(s.Frames) == 0 {
					b.deleteSession(key)
					return nil
				}
				continue
			}
			if dc.nextStepID != "" {
				nextIdx, ok := def.idx[dc.nextStepID]
				if !ok {
					return ErrInvalidInput
				}
				frame.BranchPending = false
				frame.Index = nextIdx
			} else {
				if frame.BranchPending && frame.Index == frame.BranchChoice {
					frame.Index = findNextEndIndex(def, frame.BranchAnchor)
					frame.BranchPending = false
				} else {
					frame.Index++
				}
			}
			continue
		case stepKindBranch:
			if !hasInput {
				b.saveSession(key, s)
				return nil
			}
			if step.branchHandler == nil {
				frame.Index++
				continue
			}
			nextID, err := step.branchHandler(dc)
			if err != nil {
				return err
			}
			nextIdx, ok := def.idx[nextID]
			if !ok {
				return ErrInvalidInput
			}
			branchIdx := frame.Index
			frame.Index = nextIdx
			frame.BranchAnchor = branchIdx
			frame.BranchChoice = nextIdx
			frame.BranchPending = true
			continue
		case stepKindSub:
			childDef, ok := b.dialogByName(step.subDialog)
			if !ok {
				return ErrDialogNotFound
			}
			childValues := map[string]any{}
			if step.mapIn != nil {
				childValues = step.mapIn(cloneMap(frame.Values))
			}
			frame.Index++
			if frame.BranchPending && frame.Index-1 == frame.BranchChoice {
				frame.BranchPending = false
			}
			s.Frames = append(s.Frames, dialogFrame{Dialog: childDef.name, Values: childValues, ParentSubStepID: step.id})
			continue
		case stepKindEnd:
			if step.endHandler != nil {
				if err := step.endHandler(dc); err != nil {
					return err
				}
			}
			b.popFrameToParent(s, frame)
			if len(s.Frames) == 0 {
				b.deleteSession(key)
				return nil
			}
			continue
		default:
			frame.Index++
			continue
		}
	}
}

func (b *Bot) isExpectedButtonInput(s *dialogSession, buttonID string) bool {
	if s == nil || len(s.Frames) == 0 || buttonID == "" {
		return false
	}
	frame := &s.Frames[len(s.Frames)-1]
	def, ok := b.resolveFrameDef(frame)
	if !ok || frame.Index < 0 || frame.Index >= len(def.steps) {
		return false
	}
	step := def.steps[frame.Index]
	if frame.WaitingStepID != step.id {
		return false
	}
	if len(step.clickHandlers) == 0 {
		return false
	}
	_, ok = step.clickHandlers[buttonID]
	return ok
}

func findNextEndIndex(def *DialogDef, from int) int {
	if def == nil || len(def.steps) == 0 {
		return from
	}
	for i := from + 1; i < len(def.steps); i++ {
		if def.steps[i].kind == stepKindEnd {
			return i
		}
	}
	return len(def.steps)
}

func isCloseCommand(update Update) bool {
	return strings.EqualFold(strings.TrimSpace(update.Message.Text), "/close")
}

func isBackCommand(update Update) bool {
	txt := strings.TrimSpace(update.Message.Text)
	if strings.EqualFold(txt, "/back") {
		return true
	}
	id := strings.TrimSpace(update.ButtonID)
	return strings.EqualFold(id, "back") || strings.EqualFold(id, "/back")
}

func isBlockedDialogCommand(update Update) bool {
	text := strings.TrimSpace(update.Message.Text)
	if text == "" || !strings.HasPrefix(text, "/") {
		return false
	}
	if isCloseCommand(update) || isBackCommand(update) {
		return false
	}
	// Allow dialog sentinel tokens that are valid user input.
	return !strings.EqualFold(text, "/empty")
}

func backTargetIndex(def *DialogDef, from int) int {
	if def == nil || len(def.steps) == 0 {
		return -1
	}
	target := min(from-1, len(def.steps)-1)
	for target >= 0 {
		k := def.steps[target].kind
		if k == stepKindSub || k == stepKindBranch {
			target--
			continue
		}
		break
	}
	if target < 0 {
		return 0
	}
	return target
}

func (b *Bot) stepBack(s *dialogSession) bool {
	if len(s.Frames) == 0 {
		return false
	}
	i := len(s.Frames) - 1
	frame := &s.Frames[i]
	def, ok := b.resolveFrameDef(frame)
	if !ok {
		return false
	}

	target := backTargetIndex(def, frame.Index)
	if target >= 0 && target < frame.Index {
		frame.WaitingStepID = ""
		frame.Index = target
		return true
	}

	if len(s.Frames) > 1 {
		s.Frames = s.Frames[:i]
		parent := &s.Frames[len(s.Frames)-1]
		parentDef, ok := b.resolveFrameDef(parent)
		if !ok {
			return false
		}
		target = backTargetIndex(parentDef, parent.Index)
		parent.WaitingStepID = ""
		parent.Index = target
		return true
	}

	frame.WaitingStepID = ""
	return false
}

func findBranchIndex(def *DialogDef, currentIndex int) int {
	if def == nil || len(def.steps) == 0 {
		return -1
	}
	for i := min(currentIndex, len(def.steps)-1); i >= 0; i-- {
		if def.steps[i].kind == stepKindBranch {
			return i
		}
	}
	for i := 0; i < len(def.steps); i++ {
		if def.steps[i].kind == stepKindBranch {
			return i
		}
	}
	return -1
}

func (b *Bot) rerouteByButton(s *dialogSession) bool {
	for i := len(s.Frames) - 1; i >= 0; i-- {
		frame := &s.Frames[i]
		def, ok := b.resolveFrameDef(frame)
		if !ok {
			continue
		}
		branchIdx := findBranchIndex(def, frame.Index)
		if branchIdx < 0 {
			continue
		}
		s.Frames = s.Frames[:i+1]
		frame = &s.Frames[i]
		frame.WaitingStepID = ""
		frame.Index = branchIdx
		return true
	}
	return false
}

func (b *Bot) transitionDialogFrame(s *dialogSession, frame *dialogFrame, next *Dialog, dc *dialogContext) error {
	if next == nil || next.def == nil {
		return nil
	}
	if def, ok := b.resolveFrameDef(frame); ok && def.onFinish != nil {
		if err := def.onFinish(dc); err != nil {
			return err
		}
	}
	nextDef := cloneDialogDef(next.def)
	if nextDef.name == "" {
		nextDef.name = frame.Dialog
	}
	values := cloneMap(frame.Values)
	for k, v := range nextDef.seed {
		values[k] = v
	}
	frame.Dialog = nextDef.name
	frame.RuntimeDef = nextDef
	frame.Index = 0
	frame.WaitingStepID = ""
	frame.Values = values
	frame.BranchAnchor = 0
	frame.BranchChoice = 0
	frame.BranchPending = false
	return nil
}

func (b *Bot) popFrameToParent(s *dialogSession, child *dialogFrame) {
	if len(s.Frames) == 0 {
		return
	}
	s.Frames = s.Frames[:len(s.Frames)-1]
	if len(s.Frames) == 0 {
		return
	}
	parent := &s.Frames[len(s.Frames)-1]
	mapped := cloneMap(child.Values)
	if child.ParentSubStepID != "" {
		if parentDef, ok := b.resolveFrameDef(parent); ok {
			if idx, ok := parentDef.idx[child.ParentSubStepID]; ok {
				st := parentDef.steps[idx]
				if st.mapOut != nil {
					mapped = st.mapOut(mapped)
				}
				// If subdialog was chosen right after a Branch step, skip sibling
				// subdialog options that follow it in sequence.
				if idx > 0 && parentDef.steps[idx-1].kind == stepKindBranch {
					for parent.Index < len(parentDef.steps) && parentDef.steps[parent.Index].kind == stepKindSub {
						parent.Index++
					}
				}
			}
		}
		parent.Values[child.ParentSubStepID] = mapped
	}
}

func (b *Bot) dialogByName(name string) (*DialogDef, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	d, ok := b.dialogs[name]
	return d, ok
}

func (b *Bot) resolveFrameDef(frame *dialogFrame) (*DialogDef, bool) {
	if frame != nil && frame.RuntimeDef != nil {
		return frame.RuntimeDef, true
	}
	return b.dialogByName(frame.Dialog)
}

func (b *Bot) loadSession(key string) (*dialogSession, bool) {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()
	s, ok := b.sessions[key]
	return s, ok
}

func (b *Bot) saveSession(key string, s *dialogSession) {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()
	b.sessions[key] = s
}

func (b *Bot) deleteSession(key string) {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()
	delete(b.sessions, key)
}

var _ DialogContext = (*dialogContext)(nil)
