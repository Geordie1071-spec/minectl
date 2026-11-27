package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Geordie1071-spec/minectl/internal/service"
)

type cliEvents struct {
	prefix string
	pb     *ProgressBar
	sp     *spinner
	label  string
}

func newEvents(prefix string) *cliEvents {
	return &cliEvents{prefix: prefix, label: "Working"}
}

func (e *cliEvents) Progress(percent int, msg string) {
	e.endSpinner()
	if e.pb == nil {
		e.pb = NewProgressBar(e.prefix)
	}
	e.pb.Callback(percent, msg)
}

func (e *cliEvents) Stage(label string) {
	e.finish()
	e.label = label
}

func (e *cliEvents) Log(line string) {
	e.endBar()
	if e.sp == nil {
		e.sp = startSpinner(e.label)
	}
	e.sp.setDetail(logPrefix.ReplaceAllString(strings.TrimSpace(line), ""))
}

func (e *cliEvents) Info(msg string) {
	e.finish()
	if !quiet && !jsonOut {
		fmt.Println(msg)
	}
}

func (e *cliEvents) Warn(msg string) {
	e.finish()
	if !quiet {
		fmt.Fprintln(os.Stderr, "Warning:", msg)
	}
}

func (e *cliEvents) finish() {
	e.endBar()
	e.endSpinner()
}

func (e *cliEvents) endBar() {
	if e.pb != nil {
		e.pb.End()
		e.pb = nil
	}
}

func (e *cliEvents) endSpinner() {
	if e.sp != nil {
		e.sp.end()
		e.sp = nil
	}
}

func newManager(ctx context.Context) (*service.Manager, func(), error) {
	d, err := getDockerClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	st := getStore()
	m := &service.Manager{D: d, St: st, Holder: fmt.Sprintf("cli (pid %d)", os.Getpid()), Wait: !noWait, CLIHints: true}
	return m, func() { _ = d.Close(); _ = st.Close() }, nil
}
