// package cookie implements a specialized version of a context
// - It is cancellable
// - It has a channel associated with it to reply to state callbacks
// - It can be marshalled by having a cgo Handle attached to it
package cookie

import (
	"context"
	"encoding/json"
	"runtime/cgo"

	"github.com/go-errors/errors"
)

type Cookie struct {
	c         chan string
	ctx       context.Context
	ctxCancel context.CancelFunc
	H         cgo.Handle
}

// NewWithContext creates a new cookie with a context
// It stores the cancel and channel inside of the struct
func NewWithContext(ctx context.Context) Cookie {
	ctx, cancel := context.WithCancel(ctx)
	return Cookie{
		c:         make(chan string),
		ctx:       ctx,
		ctxCancel: cancel,
	}
}

// MarshalJSON marshals the cookie to JSON
func (c *Cookie) MarshalJSON() ([]byte, error) {
	if c.H == 0 {
		return nil, errors.New("no associated handle found")
	}
	return json.Marshal(c.H)
}

// Receive receives a value from the cookie up until the context is done or errchan gets an error
// This error chan is used for goroutines to signal errors that we have to exit early
func (c *Cookie) Receive(errchan chan error) (string, error) {
	select {
	case r := <-c.c:
		return r, nil
	case e := <-errchan:
		return "", e
	case <-c.ctx.Done():
		return "", errors.WrapPrefix(context.Canceled, "receive cookie", 0)
	}
}

// Cancel cancels the cookie by calling the context cancel function
// It returns an error if no such function exists
func (c *Cookie) Cancel() error {
	if c.ctxCancel == nil {
		return errors.New("no cancel function found")
	}
	c.ctxCancel()
	return nil
}

// Send sends data to the cookie channel if the context is not canceled
func (c *Cookie) Send(data string) error {
	select {
	case <-c.ctx.Done():
		return errors.WrapPrefix(context.Canceled, "send cookie", 0)
	default:
		if c.c == nil {
			return errors.New("channel is nil")
		}
		c.c <- data
		return nil
	}
}

// Context gets the underlying context of the cookie
func (c *Cookie) Context() context.Context {
	return c.ctx
}
