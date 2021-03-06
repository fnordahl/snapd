// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016-2019 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package devicestatetest

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/asserts"
	"github.com/snapcore/snapd/httputil"
)

type DeviceServiceBehavior struct {
	ReqID string

	RequestIDURLPath string
	SerialURLPath    string

	Head          func(c *C, bhv *DeviceServiceBehavior, w http.ResponseWriter, r *http.Request)
	PostPreflight func(c *C, bhv *DeviceServiceBehavior, w http.ResponseWriter, r *http.Request)

	SignSerial func(c *C, bhv *DeviceServiceBehavior, headers map[string]interface{}, body []byte) (asserts.Assertion, error)
}

// Request IDs for hard-coded behaviors.
const (
	ReqIDFailID501          = "REQID-FAIL-ID-501"
	ReqIDBadRequest         = "REQID-BAD-REQ"
	ReqIDPoll               = "REQID-POLL"
	ReqIDSerialWithBadModel = "REQID-SERIAL-W-BAD-MODEL"
)

const (
	requestIDURLPath = "/api/v1/snaps/auth/request-id"
	serialURLPath    = "/api/v1/snaps/auth/devices"
)

func MockDeviceService(c *C, bhv *DeviceServiceBehavior) *httptest.Server {
	expectedUserAgent := httputil.UserAgent()

	// default URL paths
	if bhv.RequestIDURLPath == "" {
		bhv.RequestIDURLPath = requestIDURLPath
		bhv.SerialURLPath = serialURLPath
	}

	var mu sync.Mutex
	count := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		default:
			c.Fatalf("unexpected verb %q", r.Method)
		case "HEAD":
			if r.URL.Path != "/" {
				c.Fatalf("unexpected HEAD request %q", r.URL.String())
			}
			if bhv.Head != nil {
				bhv.Head(c, bhv, w, r)
			}
			w.WriteHeader(200)
			return
		case "POST":
			// carry on
		}

		if bhv.PostPreflight != nil {
			bhv.PostPreflight(c, bhv, w, r)
		}

		switch r.URL.Path {
		default:
			c.Fatalf("unexpected POST request %q", r.URL.String())
		case bhv.RequestIDURLPath:
			if bhv.ReqID == ReqIDFailID501 {
				w.WriteHeader(501)
				return
			}
			w.WriteHeader(200)
			c.Check(r.Header.Get("User-Agent"), Equals, expectedUserAgent)
			io.WriteString(w, fmt.Sprintf(`{"request-id": "%s"}`, bhv.ReqID))
		case bhv.SerialURLPath:
			c.Check(r.Header.Get("User-Agent"), Equals, expectedUserAgent)

			mu.Lock()
			serialNum := 9999 + count
			count++
			mu.Unlock()

			b, err := ioutil.ReadAll(r.Body)
			c.Assert(err, IsNil)
			a, err := asserts.Decode(b)
			c.Assert(err, IsNil)
			serialReq, ok := a.(*asserts.SerialRequest)
			c.Assert(ok, Equals, true)
			err = asserts.SignatureCheck(serialReq, serialReq.DeviceKey())
			c.Assert(err, IsNil)
			brandID := serialReq.BrandID()
			model := serialReq.Model()
			reqID := serialReq.RequestID()
			if reqID == ReqIDBadRequest {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(400)
				w.Write([]byte(`{
  "error_list": [{"message": "bad serial-request"}]
}`))
				return
			}
			if reqID == ReqIDPoll && serialNum != 10002 {
				w.WriteHeader(202)
				return
			}
			serialStr := fmt.Sprintf("%d", serialNum)
			if serialReq.Serial() != "" {
				// use proposed serial
				serialStr = serialReq.Serial()
			}
			serial, err := bhv.SignSerial(c, bhv, map[string]interface{}{
				"authority-id":        "canonical",
				"brand-id":            brandID,
				"model":               model,
				"serial":              serialStr,
				"device-key":          serialReq.HeaderString("device-key"),
				"device-key-sha3-384": serialReq.SignKeyID(),
				"timestamp":           time.Now().Format(time.RFC3339),
			}, serialReq.Body())
			c.Assert(err, IsNil)
			w.Header().Set("Content-Type", asserts.MediaType)
			w.WriteHeader(200)
			encoded := asserts.Encode(serial)
			if reqID == ReqIDSerialWithBadModel {
				encoded = bytes.Replace(encoded, []byte("model: pc"), []byte("model: bad-model-foo"), 1)
			}
			w.Write(encoded)
		}
	}))
}
