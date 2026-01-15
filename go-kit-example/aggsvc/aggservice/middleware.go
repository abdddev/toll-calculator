package aggservice

import (
	"context"
	"time"

	"github.com/abdddev/toll-calculator/types"
	"github.com/go-kit/log"
)

// Middleware — функция, которая оборачивает Service и возвращает новый Service
type Middleware func(Service) Service

// loggingMiddleware — middleware для логирования вызовов сервиса
type loggingMiddleware struct {
	log  log.Logger // логгер
	next Service    // следующий сервис в цепочке
}

// newLoggingMiddleware — фабрика logging middleware
func newLoggingMiddleware(logger log.Logger) Middleware {
	// возвращаем функцию, которая принимает Service
	return func(next Service) Service {
		// и возвращает обёрнутый сервис
		return loggingMiddleware{
			next: next,
			log:  logger,
		}
	}
}

// Aggregate — логирующая обёртка метода Aggregate
func (mw loggingMiddleware) Aggregate(ctx context.Context, dist types.Distance) (err error) {
	defer func(start time.Time) {
		mw.log.Log("method", "Aggregate", "took", time.Since(start), "obu", dist.OBUID, "distance", dist.Value, "err", err)
	}(time.Now())
	err = mw.next.Aggregate(ctx, dist)
	return
}

// Calculate — логирующая обёртка метода Calculate
func (mw loggingMiddleware) Calculate(ctx context.Context, dist int) (inv *types.Invoice, err error) {
	defer func(start time.Time) {
		mw.log.Log("took", time.Since(start), "dist", dist, "inv", inv, "err", err)
	}(time.Now())
	inv, err = mw.next.Calculate(ctx, dist)
	return
}

// instrumentationMiddleware — middleware для метрик / трассировки
type instrumentationMiddleware struct {
	next Service
}

// newInstrumentationMiddleware — фабрика instrumentation middleware
func newInstrumentationMiddleware() Middleware {
	return func(next Service) Service {
		return instrumentationMiddleware{
			next: next,
		}
	}
}

// Aggregate — прокидывает вызов дальше (заглушка под метрики)
func (mw instrumentationMiddleware) Aggregate(ctx context.Context, dist types.Distance) error {
	return mw.next.Aggregate(ctx, dist)
}

// Calculate — прокидывает вызов дальше (заглушка под метрики)
func (mw instrumentationMiddleware) Calculate(ctx context.Context, dist int) (*types.Invoice, error) {
	return mw.next.Calculate(ctx, dist)
}
