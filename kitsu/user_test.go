package kitsu

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestUserService_Show(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/"+defaultAPIVersion+"users/29745", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		fmt.Fprintf(w, `{"data":{"id":"29745","type":"users","attributes":{"name":"chitanda","lifeSpentOnAnime":550}}}`)
	})

	got, _, err := client.User.Show("29745")
	if err != nil {
		t.Errorf("User.Show returned error: %v", err)
	}

	want := &User{ID: "29745", Name: "chitanda", LifeSpent: 550}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("User.Show user mismatch\nhave: %#+v\nwant: %#+v", got, want)
	}
}

func TestUserService_Show_notFound(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/"+defaultAPIVersion+"users/0", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		http.Error(w, `{"errors":[{"title":"Record not found","detail":"The record identified by 0 could not be found.","code":"404","status":"404"}]}`, http.StatusNotFound)
	})

	_, resp, err := client.User.Show("0")
	if err == nil {
		t.Error("Expected HTTP 404 error.")
	}

	if resp == nil {
		t.Error("Expected to return HTTP response despite the API error.")
	}
}

func TestUserService_List(t *testing.T) {
	setup()
	defer teardown()

	mux.HandleFunc("/"+defaultAPIVersion+"users", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		testHeader(t, r, "Accept", defaultMediaType)
		testFormValues(t, r, values{
			"page[limit]":  "2",
			"page[offset]": "0",
			"filter[name]": "vikhyat",
			"sort":         "-followersCount",
			"include":      "libraryEntries",
		})

		const s = `
		{
			"data": [{
				"id": "1",
				"type": "users",
				"attributes": {
					"name": "vikhyat"
				}
			}],
			"links": {
				"first": "https://kitsu.io/api/edge/users?filter%5Bname%5D=gokapaya&page%5Blimit%5D=2&page%5Boffset%5D=0&sort=-followersCount",
				"last": "https://kitsu.io/api/edge/users?filter%5Bname%5D=gokapaya&page%5Blimit%5D=2&page%5Boffset%5D=0&sort=-followersCount"
			}
		}
		`
		fmt.Fprint(w, s)
	})

	opt := &Options{
		PageLimit:  2,
		PageOffset: 0,
		Filter:     "name",
		FilterVal:  []string{"vikhyat"},
		Sort:       []string{"-followersCount"},
		Include:    []string{"libraryEntries"},
	}

	got, resp, err := client.User.List(opt)
	if err != nil {
		t.Errorf("User.List returned error: %v", err)
	}

	want := []*User{
		{ID: "1", Name: "vikhyat"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("User.List mismatch\nhave: %#+v\nwant: %#+v", got, want)
		data, _ := json.Marshal(got)
		fmt.Println(string(data))
		data, _ = json.Marshal(want)
		fmt.Println(string(data))
	}
	if got, want := resp.FirstOffset, 0; got != want {
		t.Errorf("Anime.List response FirstOffset = %d, want %d", got, want)
	}
	if got, want := resp.LastOffset, 0; got != want {
		t.Errorf("Anime.List response LastOffset = %d, want %d", got, want)
	}
	if got, want := resp.NextOffset, 0; got != want {
		t.Errorf("Anime.List response NextOffset = %d, want %d", got, want)
	}
	if got, want := resp.PrevOffset, 0; got != want {
		t.Errorf("Anime.List response PrevOffset = %d, want %d", got, want)
	}
}
