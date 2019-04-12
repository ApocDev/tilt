package tracer

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lightstep/lightstep-tracer-go"
	"github.com/windmilleng/tilt/internal/logger"

	"github.com/pkg/errors"

	"github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
)

const windmillTracerHostPort = "opentracing.windmill.build:9411"

type TracerBackend int

const (
	Windmill TracerBackend = iota
	Lightstep
)

type zipkinLogger struct {
	ctx context.Context
}

func (zl zipkinLogger) Log(keyvals ...interface{}) error {
	logger.Get(zl.ctx).Debugf("%v", keyvals)
	return nil
}

var _ zipkin.Logger = zipkinLogger{}

func Init(ctx context.Context, tracer TracerBackend) (func() error, error) {
	if tracer == Lightstep {
		return initLightStep(ctx)
	}
	return initWindmillZipkin(ctx)
}

func TraceID(ctx context.Context) (string, error) {
	spanContext := opentracing.SpanFromContext(ctx)
	if spanContext == nil {
		return "", errors.New("cannot get traceid - there is no span context")
	}
	zipkinSpanContext, ok := spanContext.Context().(zipkin.SpanContext)
	if !ok {
		return "", errors.New("cannot get traceid - span context was not a zipkin span context")
	}
	return zipkinSpanContext.TraceID.ToHex(), nil
}

// TagStrToMap converts a user-passed string of tags of the form `key1=val1,key2=val2` to a map.
func TagStrToMap(tagStr string) map[string]string {
	if tagStr == "" {
		return nil
	}

	res := make(map[string]string)
	pairs := strings.Split(tagStr, ",")
	for _, p := range pairs {
		elems := strings.Split(strings.TrimSpace(p), "=")
		if len(elems) != 2 {
			log.Printf("got malformed trace tag: %s", p)
			continue
		}
		res[elems[0]] = elems[1]
	}
	return res
}

func StringToTracerBackend(s string) (TracerBackend, error) {
	switch s {
	case "windmill":
		return Windmill, nil
	case "lightstep":
		return Lightstep, nil
	default:
		return Windmill, fmt.Errorf("Invalid Tracer backend: %s", s)
	}
}

func initWindmillZipkin(ctx context.Context) (func() error, error) {
	collector, err := zipkin.NewHTTPCollector(fmt.Sprintf("http://%s/api/v1/spans", windmillTracerHostPort), zipkin.HTTPLogger(zipkinLogger{ctx}))

	if err != nil {
		return nil, errors.Wrap(err, "unable to create zipkin collector")
	}

	recorder := zipkin.NewRecorder(collector, true, "0.0.0.0:0", "tilt")
	tracer, err := zipkin.NewTracer(recorder)

	if err != nil {
		return nil, errors.Wrap(err, "unable to create tracer")
	}

	opentracing.SetGlobalTracer(tracer)

	return collector.Close, nil
}

func initLightStep(ctx context.Context) (func() error, error) {
	token, ok := os.LookupEnv("LIGHTSTEP_ACCESS_TOKEN")
	if !ok {
		return nil, fmt.Errorf("No token found in the LIGHTSTEP_ACCESS_TOKEN environment variable")
	}
	lightstepTracer := lightstep.NewTracer(lightstep.Options{
		AccessToken: token,
	})

	opentracing.SetGlobalTracer(lightstepTracer)

	close := func() error {
		lightstepTracer.Close(context.Background())
		return nil
	}
	return close, nil
}
