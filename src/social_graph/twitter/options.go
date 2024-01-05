package twitter

import "github.com/go-resty/resty/v2"

// ApiRequestOption is provided for modifying the requests
type ApiRequestOption interface {
	Apply(*resty.Request)
}

type OptApplyQueryParam struct {
	Key   string
	Value string
}

func (o *OptApplyQueryParam) Apply(req *resty.Request) {
	req.SetQueryParam(o.Key, o.Value)
}

// OptApplyPaginationToken applies the next pagination page token parameter.
func OptApplyPaginationToken(paginationToken string) ApiRequestOption {
	return &OptApplyQueryParam{
		Key:   "pagination_token",
		Value: paginationToken,
	}
}

// OptApplyMaxResults applies maximum results per page parameter. maxResult
// cannot be less than 5 or greater than 100.
func OptApplyMaxResults(maxResult string) ApiRequestOption {
	return &OptApplyQueryParam{
		Key:   "max_results",
		Value: maxResult,
	}
}
