package entitlement

import (
	"context"
	"sync"
	"time"

	secure "cue-go-remix/internal/entitlement/platform"
)

type Options struct {
	BaseURL   string
	AllowHTTP bool
	MachineID func() (string, error)
	Load      func() (secure.Credential, error)
	Save      func(secure.Credential) error
	Delete    func() error
	Changed   func(Status)
	Trace     func(string, ...any)
	Now       func() time.Time
}

type Manager struct {
	mu          sync.Mutex
	op          sync.Mutex
	opts        Options
	client      *Client
	root        context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	started     bool
	closed      bool
	initialized bool
	device      string
	credential  secure.Credential
	token       string
	state       Status
	work        context.Context
	revoke      context.CancelFunc
	lastOK      time.Time
	expires     time.Time
	next        time.Time
	retry       bool
	epoch       uint64
}

func New(parent context.Context, o Options) *Manager {
	if o.MachineID == nil {
		o.MachineID = secure.MachineID
	}
	if o.Load == nil {
		o.Load = secure.Load
	}
	if o.Save == nil {
		o.Save = secure.Save
	}
	if o.Delete == nil {
		o.Delete = secure.Delete
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	ctx, cancel := context.WithCancel(parent)
	client := NewClient(o.BaseURL, o.AllowHTTP)
	client.trace = o.Trace
	client.log("event=license-config origin=%q valid=%t http_allowed=%t", client.origin, client.err == nil, o.AllowHTTP)
	return &Manager{opts: o, client: client, root: ctx, cancel: cancel, state: Status{State: "checking", Message: "正在检查本机授权"}}
}
