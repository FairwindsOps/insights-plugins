//go:build linux

package agent

import (
	"context"
	"fmt"
	"log/slog"

	gadgetcontext "github.com/inspektor-gadget/inspektor-gadget/pkg/gadget-context"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/operators"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/operators/simple"
	grpcruntime "github.com/inspektor-gadget/inspektor-gadget/pkg/runtime/grpc"
)

func runGadget(
	ctx context.Context,
	cfg GadgetConfig,
	log *slog.Logger,
	gadgetName string,
	onInit func(operators.GadgetContext) error,
	gadgetParams map[string]string,
) error {
	flowOp := simple.New("flow-collector", simple.OnInit(func(gctx operators.GadgetContext) error {
		return onInit(gctx)
	}))

	gctx := gadgetcontext.New(
		ctx,
		cfg.GadgetImage,
		gadgetcontext.WithDataOperators(flowOp),
	)

	rt := grpcruntime.New()
	params := rt.GlobalParamDescs().ToParams()
	if err := params.Get(grpcruntime.ParamRemoteAddress).Set(cfg.IGAddress); err != nil {
		return fmt.Errorf("set remote address: %w", err)
	}
	if err := rt.Init(params); err != nil {
		return fmt.Errorf("runtime init: %w", err)
	}
	defer func() {
		if err := rt.Close(); err != nil {
			log.Error("close runtime", "err", err)
		}
	}()

	log.Info("starting gadget", "gadget", gadgetName, "ig", cfg.IGAddress, "image", cfg.GadgetImage)
	if err := rt.RunGadget(gctx, nil, gadgetParams); err != nil {
		return fmt.Errorf("run %s: %w", gadgetName, err)
	}
	return nil
}
