package aggservice

import (
	"context"

	"github.com/abdddev/toll-calculator/types"
	"github.com/go-kit/log"
)

const basePrice = 3.15

// Service — интерфейс бизнес-логики сервиса
type Service interface {
	Aggregate(context.Context, types.Distance) error
	Calculate(context.Context, int) (*types.Invoice, error)
}

// Storer — интерфейс хранилища
type Storer interface {
	Insert(types.Distance) error
	Get(int) (float64, error)
}

// BasicService — базовая реализация Service
type BasicService struct {
	store Storer // хранилище (memory / db / etc)
}

// конструктор базового сервиса
func newBasicService(store Storer) Service {
	return &BasicService{
		store: store,
	}
}

// Aggregate — просто сохраняет дистанцию
func (svc *BasicService) Aggregate(_ context.Context, dist types.Distance) error {
	return svc.store.Insert(dist)
}

// Calculate — считает инвойс
func (svc *BasicService) Calculate(_ context.Context, obuID int) (*types.Invoice, error) {
	dist, err := svc.store.Get(obuID)
	if err != nil {
		return nil, err
	}
	inv := &types.Invoice{
		OBUID:         obuID,
		TotalDistance: dist,
		TotalAmount:   basePrice * dist,
	}
	return inv, nil
}

// New — собирает сервис целиком (service + middleware)
func New(logger log.Logger) Service {
	var svc Service
	{
		// базовый сервис с memory store
		svc = newBasicService(NewMemoryStore())
		// middleware логирования
		svc = newLoggingMiddleware(logger)(svc)
		// middleware метрик / трассировки
		svc = newInstrumentationMiddleware()(svc)
	}
	return svc
}
