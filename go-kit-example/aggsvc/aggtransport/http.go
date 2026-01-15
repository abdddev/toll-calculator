package aggtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abdddev/toll-calculator/go-kit-example/aggsvc/aggendpoint"
	"github.com/abdddev/toll-calculator/go-kit-example/aggsvc/aggservice"
	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/go-kit/log"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

// NewHTTPClient — HTTP-клиент, который возвращает Service
// но внутри ходит по HTTP к удалённому сервису
func NewHTTPClient(instance string, logger log.Logger) (aggservice.Service, error) {
	// если забыли указать схему — добавляем http://
	if !strings.HasPrefix(instance, "http") {
		instance = "http://" + instance
	}
	u, err := url.Parse(instance)
	if err != nil {
		return nil, err
	}

	// Создаём один общий rate limiter для клиента.
	// Он ограничивает суммарный исходящий QPS этого клиента
	// ко ВСЕМ методам удалённого сервиса.
	//
	// Также ниже мы создаём circuit breaker для каждого endpoint’а отдельно.
	// Это сделано для примера — при желании можно использовать
	// один общий circuit breaker на весь удалённый сервис.
	limiter := ratelimit.NewErroringLimiter(rate.NewLimiter(rate.Every(time.Second), 100))

	// Общие middleware / опции HTTP-клиента
	var options []httptransport.ClientOption

	// Каждый endpoint — это HTTP-клиент (transport/http.Client),
	// который реализует интерфейс endpoint.Endpoint.
	//
	// Каждый такой endpoint оборачивается в набор middleware
	// (rate limit, circuit breaker и т.д.).
	//
	// Если бы это была отдельная клиентская библиотека,
	// вся эта логика жила бы именно здесь, чтобы сервер
	// всегда получал единообразное поведение клиента.
	var aggregateEndpoint endpoint.Endpoint
	{
		aggregateEndpoint = httptransport.NewClient(
			"POST",
			copyURL(u, "/aggregate"),
			encodeHTTPGenericRequest,
			decodeHTTPAggregateResponse,
			options...,
		).Endpoint()
		aggregateEndpoint = limiter(aggregateEndpoint)
		aggregateEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Aggregate",
			Timeout: 30 * time.Second,
		}))(aggregateEndpoint)
	}

	// Calculate endpoint — то же самое, что и Aggregate,
	// но с немного другими middleware.
	// Это сделано, чтобы показать, что каждый endpoint
	// можно настраивать индивидуально.
	var calculateEndpoint endpoint.Endpoint
	{
		calculateEndpoint = httptransport.NewClient(
			"POST",
			copyURL(u, "/invoice"),
			encodeHTTPGenericRequest,
			decodeHTTPCalculateResponse,
			options...,
		).Endpoint()
		calculateEndpoint = limiter(calculateEndpoint)
		calculateEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Concat",
			Timeout: 10 * time.Second,
		}))(calculateEndpoint)
	}

	// Возвращаем aggendpoint.Set как aggservice.Service.
	// Это возможно потому, что Set реализует методы Service.
	// По сути — простой glue-код между endpoint и service.
	return aggendpoint.Set{
		AggregateEndpoint: aggregateEndpoint,
		CalculateEndpoint: calculateEndpoint,
	}, nil
}

func errorEncoder(ctx context.Context, err error, w http.ResponseWriter) {
	fmt.Println("this is coming from the error encoder ->", err)
}

func NewHTTPHandler(endpoints aggendpoint.Set, logger log.Logger) http.Handler {
	options := []httptransport.ServerOption{
		httptransport.ServerErrorEncoder(errorEncoder),
		httptransport.ServerErrorHandler(transport.NewLogErrorHandler(logger)),
	}
	m := http.NewServeMux()
	m.Handle("/aggregate", httptransport.NewServer(
		endpoints.AggregateEndpoint,
		decodeHTTPAggregateRequest,
		encodeHTTPGenericResponse,
		options...,
	))
	m.Handle("/invoice", httptransport.NewServer(
		endpoints.CalculateEndpoint,
		decodeHTTPCalculateRequest,
		encodeHTTPGenericResponse,
		options...,
	))
	return m
}

func encodeHTTPGenericResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if f, ok := response.(endpoint.Failer); ok && f.Failed() != nil {
		errorEncoder(ctx, f.Failed(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

func decodeHTTPAggregateRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req aggendpoint.AggregateRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

func decodeHTTPCalculateRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var req aggendpoint.CalculateRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

func decodeHTTPCalculateResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Status)
	}
	var resp aggendpoint.CalculateResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func decodeHTTPAggregateResponse(_ context.Context, r *http.Response) (interface{}, error) {
	if r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Status)
	}
	var resp aggendpoint.AggregateResponse
	err := json.NewDecoder(r.Body).Decode(&resp)
	return resp, err
}

func copyURL(base *url.URL, path string) *url.URL {
	next := *base
	next.Path = path
	return &next
}

// encodeHTTPGenericRequest is a transport/http.EncodeRequestFunc that
// JSON-encodes any request to the request body. Primarily useful in a client.
func encodeHTTPGenericRequest(_ context.Context, r *http.Request, request interface{}) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(request); err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(&buf)
	return nil
}
