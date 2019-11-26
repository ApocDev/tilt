package engine

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/hud"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

const profileFileName = "tilt.profile"

func (p *ProfilerManager) Start(ctx context.Context, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return errors.Wrapf(err, "error creating profile file %s", filename)
	}
	p.f = f
	err = pprof.StartCPUProfile(f)
	if err != nil {
		return errors.Wrap(err, "error starting cpu profile")
	}
	logger.Get(ctx).Infof("starting pprof profile to %s", profileFileName)

	return nil
}

func (p *ProfilerManager) Stop(ctx context.Context) error {
	pprof.StopCPUProfile()
	err := p.f.Close()
	if err != nil {
		return errors.Wrap(err, "error closing profile file")
	}
	logger.Get(ctx).Infof("stopped pprof profile to %s", profileFileName)
	p.f = nil

	return nil
}

type ProfilerManager struct {
	dispatched  bool
	isProfiling bool
	f           *os.File
}

func (p *ProfilerManager) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()

	if !p.dispatched {
		p.dispatched = true
		go func() {
			time.Sleep(time.Second * 15)
			fmt.Println("Starting auto-profile... ✅")
			st.Dispatch(hud.StartProfilingAction{})
			time.Sleep(time.Second * 10)
			fmt.Println("Stopping auto-profile... 🛑")
			st.Dispatch(hud.StopProfilingAction{})
		}()
	}
	if p.isProfiling == state.IsProfiling {
		return
	}
	p.isProfiling = state.IsProfiling

	if p.isProfiling {
		err := p.Start(ctx, profileFileName)
		if err != nil {
			st.Dispatch(NewErrorAction(err))
		}
	} else {
		err := p.Stop(ctx)
		if err != nil {
			st.Dispatch(NewErrorAction(err))
		}
	}
}

func NewProfilerManager() *ProfilerManager {
	return &ProfilerManager{}
}
