package aggregator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Error values describing why an id could not be fetched. Their text is what
// clients see in the "errors" array.
var (
	errTimeout     = errors.New("upstream timeout")
	errUnreachable = errors.New("upstream unreachable")
	errInvalidJSON = errors.New("upstream returned invalid JSON")
)

// statusError is a non-2xx answer from the upstream.
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("upstream status %d", e.code) }

// fetch returns the JSON body of GET {UpstreamURL}/items/{id}.
//
// TODO:
//   - give every attempt its own timeout (cfg.CallTimeout) derived from ctx;
//   - retry up to cfg.MaxRetries times on network errors, timeouts and 5xx,
//     waiting BackoffBase, 2*BackoffBase, ... between attempts (the wait must
//     stop as soon as ctx is done);
//   - never retry 4xx or an invalid JSON body;
//   - return statusError / errTimeout / errUnreachable / errInvalidJSON so the
//     message matches "upstream status 500" etc.;
//   - stop at once and return ctx.Err() when ctx itself is cancelled;
//   - always close (and drain) the response body.
func (s *Service) fetch(ctx context.Context, id int) (json.RawMessage, error) {
	return nil, errors.New("TODO: fetch is not implemented")
}
