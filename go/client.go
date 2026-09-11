package asks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/openabstractions/abstraction-identity/listen"
)

var ErrNoService = errors.New("asks: no service is listening")

type Client struct {
	Endpoint string
	Admin    string
}

func DefaultEndpoint() string { return listen.Endpoint("asks") }

func DefaultStateDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "openabstractions", "asks")
}

func (c *Client) do(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	req.Admin = c.Admin
	nc, err := listen.Dial(c.Endpoint)
	if err != nil {
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		return Response{}, fmt.Errorf("%w at %s (%v)", ErrNoService, c.Endpoint, err)
	}
	defer nc.Close()
	return exchange(ctx, nc, req)
}

func exchange(ctx context.Context, nc net.Conn, req Request) (Response, error) {
	stop := context.AfterFunc(ctx, func() { nc.Close() })
	defer stop()
	raw, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	if _, err := nc.Write(append(raw, '\n')); err != nil {
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		return Response{}, err
	}
	return read(ctx, nc)
}

func read(ctx context.Context, nc net.Conn) (Response, error) {
	sc := bufio.NewScanner(nc)
	sc.Buffer(make([]byte, maxLine), maxLine)
	if !sc.Scan() {
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		if err := sc.Err(); err != nil {
			return Response{}, err
		}
		return Response{}, errors.New("asks: the service closed the connection")
	}
	var resp Response
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		return Response{}, err
	}
	if err := resp.Err(); err != nil {
		return Response{}, err
	}
	return resp, nil
}

func (c *Client) Ask(ctx context.Context, a Ask) (Answer, error) {
	return c.ask(ctx, a, false)
}

func (c *Client) Await(ctx context.Context, a Ask) (Answer, error) {
	return c.ask(ctx, a, true)
}

func (c *Client) ask(ctx context.Context, a Ask, wait bool) (Answer, error) {
	resp, err := c.do(ctx, Request{Op: OpAsk, Ask: &a, Wait: wait})
	if err != nil {
		return Answer{}, err
	}
	return *resp.Answer, nil
}

func (c *Client) Pending() ([]Record, error) {
	resp, err := c.do(context.Background(), Request{Op: OpPending})
	return resp.Records, err
}

func (c *Client) Answered() ([]Record, error) {
	resp, err := c.do(context.Background(), Request{Op: OpAnswered})
	return resp.Records, err
}

func (c *Client) Answer(id, option string) (Record, error) {
	resp, err := c.do(context.Background(), Request{Op: OpAnswer, ID: id, Option: option})
	if err != nil {
		return Record{}, err
	}
	return resp.Records[0], nil
}

func (c *Client) Forget(id string) error {
	_, err := c.do(context.Background(), Request{Op: OpForget, ID: id})
	return err
}
