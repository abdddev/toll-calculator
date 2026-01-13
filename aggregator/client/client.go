package client

import (
	"context"

	"github.com/abdddev/toll-calculator/types"
)

type Client interface {
	Aggregate(context.Context, *types.AggregateRequest) error
	GetInvoice(context.Context, int) (*types.Invoice, error)
	//AggregateInvoice(req types.Distance) error
}
