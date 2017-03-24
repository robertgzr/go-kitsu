package kitsu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/nstratos/jsonapi"
)

const (
	defaultBaseURL    = "https://kitsu.io/"
	defaultAPIVersion = "api/edge/"

	defaultMediaType = "application/vnd.api+json"
)

// Client manages communication with the kitsu.io API.
type Client struct {
	client *http.Client

	BaseURL *url.URL

	common service

	Anime *AnimeService
	User  *UserService
}

type service struct {
	client *Client
}

// Resource represent a JSON API resource object. It contains common fields
// used by the Kitsu API resources like Anime and Manga.
//
// JSON API docs: http://jsonapi.org/format/#document-resource-objects
type Resource struct {
	ID    string `json:"id"`
	Type  string `json:"type,omitempty"`
	Links Link   `json:"links,omitempty"`
}

// Link represent links that may be contained by resource objects. According to
// the current Kitsu API documentation, links are represented as a string.
//
// JSON API docs: http://jsonapi.org/format/#document-links
type Link struct {
	Self string `json:"self"`
}

// NewClient returns a new kitsu.io API client.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	baseURL, _ := url.Parse(defaultBaseURL)

	c := &Client{client: httpClient, BaseURL: baseURL}

	c.common.client = c

	c.Anime = (*AnimeService)(&c.common)
	c.User = (*UserService)(&c.common)

	return c
}

// urlOption allows to specify URL parameters to the Kitsu API to change the
// data that will be retrieved.
type urlOption func(v *url.Values)

// Pagination allows to choose how many pages of a resource to receive by
// specifying pagination parameters limit and offset. Resources are paginated
// by default.
func Pagination(limit, offset int) urlOption {
	return func(v *url.Values) {
		v.Set("page[limit]", strconv.Itoa(limit))
		v.Set("page[offset]", strconv.Itoa(offset))
	}
}

// Limit allows to control the number of results that will be retrieved. It can
// be used together with Offset to control the pagination results. Results have
// a default limit.
func Limit(limit int) urlOption {
	return func(v *url.Values) {
		v.Set("page[limit]", strconv.Itoa(limit))
	}
}

// Offset is meant to be used together with Limit and allows to control the
// offset of the pagination.
func Offset(offset int) urlOption {
	return func(v *url.Values) {
		v.Set("page[offset]", strconv.Itoa(offset))
	}
}

// Filter allows to query data that contains certain matching attributes or
// relationships. For example, to retrieve all the anime of the Action genre,
// "genres" can be passed as the attribute and "action" as one of the values
// likes so:
//
//     Filter("genres", "action").
//
// Many values can be provided to be filtered like so:
//
//     Filter("genres", "action", "drama").
//
func Filter(attribute string, values ...string) urlOption {
	return func(v *url.Values) {
		v.Set(fmt.Sprintf("filter[%s]", attribute), strings.Join(values, ","))
	}
}

// Search can be passed as an option and allows to search for media based on
// query text.
func Search(query string) urlOption {
	return func(v *url.Values) {
		v.Set("filter[text]", query)
	}
}

// Sort can be specified to provide sorting for one or more attributes. By default, sorts are applied in ascending order.
// For descending order a - can be prepended to the sort parameter (e.g.
// -averageRating for Anime).
//
// For example to sort by the attribute "averageRating" of Anime:
//
//    Sort("averageRating")
//
// And for descending order:
//
//    Sort("-averageRating")
//
// Many sort parameters can be specified:
//
//    Sort("followersCount", "-followingCount")
//
func Sort(attributes ...string) urlOption {
	return func(v *url.Values) {
		v.Set("sort", strings.Join(attributes, ","))
	}
}

// Include allows to include one or more related resources by specifying the
// relationships and successive relationships using a dot notation. For example
// for Anime to also include Casting:
//
//    Include("castings")
//
// If Casting is needed to also include Person and Character:
//
//    Include("castings.character", "castings.person"),
//
func Include(relationships ...string) urlOption {
	return func(v *url.Values) {
		v.Set("include", strings.Join(relationships, ","))
	}
}

// NewRequest creates an API request. If a relative URL is provided in urlStr,
// it will be resolved relative to the BaseURL of the Client. Relative URLs
// should always be specified without a preceding slash. If body is specified,
// it will be encoded to JSON and used as the request body.
func (c *Client) NewRequest(method, urlStr string, body interface{}, opts ...urlOption) (*http.Request, error) {
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	v := rel.Query()
	if opts != nil {
		for _, opt := range opts {
			if opt != nil { // Avoid panic in case the user passes a nil option.
				opt(&v)
			}
		}
	}
	rel.RawQuery = v.Encode()

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		encErr := json.NewEncoder(buf).Encode(body)
		if encErr != nil {
			return nil, encErr
		}
	}

	u := c.BaseURL.ResolveReference(rel)

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-type", defaultMediaType)
	}
	req.Header.Set("Accept", defaultMediaType)

	return req, nil
}

// Response is a Kitsu API response. It wraps the standard http.Response
// returned from the request and provides access to pagination offsets for
// responses that return an array of results.
type Response struct {
	*http.Response

	NextOffset  int
	PrevOffset  int
	FirstOffset int
	LastOffset  int
}

func newResponse(r *http.Response) *Response {
	return &Response{Response: r}
}

// Do sends an API request and returns the API response. If an API error has
// occurred both the response and the error will be returned in case the caller
// wishes to further inspect the response. If v is passed as an argument, then
// the API response is JSON decoded and stored to v.
func (c *Client) Do(req *http.Request, v interface{}) (*Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		return newResponse(resp), err
	}

	if v != nil {
		err = jsonapi.UnmarshalPayload(resp.Body, v)
	}
	return newResponse(resp), err
}

func (c *Client) DoMany(req *http.Request, t reflect.Type) ([]interface{}, *Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		return nil, newResponse(resp), err
	}

	var v []interface{}
	var links *jsonapi.Links
	v, links, err = jsonapi.UnmarshalManyPayloadWithLinks(resp.Body, t)
	if err != nil {
		return nil, newResponse(resp), err
	}

	o, err := parseOffset(*links)
	if err != nil {
		return nil, newResponse(resp), err
	}
	response := &Response{
		Response:    resp,
		FirstOffset: o.first,
		LastOffset:  o.last,
		PrevOffset:  o.prev,
		NextOffset:  o.next,
	}
	return v, response, err
}

// ErrorResponse reports one or more errors caused by an API request.
type ErrorResponse struct {
	Response *http.Response // HTTP response that caused this error
	Errors   []Error        `json:"errors"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d %+v",
		r.Response.Request.Method, r.Response.Request.URL,
		r.Response.StatusCode, r.Errors)
}

// Error holds the details of each invidivual error in an ErrorResponse.
//
// JSON API docs: http://jsonapi.org/format/#error-objects
type Error struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%v: error %v: %v(%v)",
		e.Status, e.Code, e.Title, e.Detail)
}

func checkResponse(r *http.Response) error {
	if c := r.StatusCode; 200 <= c && c <= 299 {
		return nil
	}
	errorResponse := &ErrorResponse{Response: r}
	body, err := ioutil.ReadAll(r.Body)
	if err == nil && body != nil {
		json.Unmarshal(body, errorResponse)
	}
	return errorResponse
}
