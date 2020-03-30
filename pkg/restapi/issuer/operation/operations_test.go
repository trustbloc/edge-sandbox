/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package operation

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/trustbloc/edge-sandbox/pkg/token"
)

const authHeader = "Bearer ABC"

const testCredentialRequest = `{
"context":["https://www.w3.org/2018/credentials/examples/v1"],
"type": [
    "VerifiableCredential",
    "UniversityDegreeCredential"
  ],
  "credentialSubject": {
    "id": "did:example:ebfeb1f712ebc6f1c276e12ec21",
    "degree": {
      "type": "BachelorDegree",
      "university": "MIT"
    },
    "name": "Jayden Doe",
    "spouse": "did:example:c276e12ec21ebfeb1f712ebc6f1"
  },
  "profile": "test"
}`

const foo = `{"id":1,"userid":"100","name":"Foo Bar","email":"foo@bar.com"}`
const jsonArray = `[{}]`

func TestOperation_Login(t *testing.T) {
	cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{}}
	handler := getHandlerWithConfig(t, login, cfg)

	buff, status, err := handleRequest(handler, nil, login, true)
	require.NoError(t, err)
	require.Contains(t, buff.String(), "vcs profile is empty")
	require.Equal(t, http.StatusBadRequest, status)

	buff, status, err = handleRequest(handler, nil, login+"?vcsProfile=vc-issuer-1", true)
	require.NoError(t, err)
	require.Contains(t, buff.String(), "Temporary Redirect")
	require.Equal(t, http.StatusTemporaryRedirect, status)

	buff, status, err = handleRequest(handler, nil, login+"?scope=test&vcsProfile=vc-issuer-1", true)
	require.NoError(t, err)
	require.Contains(t, buff.String(), "Temporary Redirect")
	require.Equal(t, http.StatusTemporaryRedirect, status)
}

func TestOperation_Login3(t *testing.T) {
	cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{}}
	handler := getHandlerWithConfig(t, login, cfg)

	req, err := http.NewRequest(handler.Method(), login+"?scope=test&vcsProfile=vc-issuer-1", bytes.NewBuffer([]byte("")))
	require.NoError(t, err)

	router := mux.NewRouter()
	router.HandleFunc(handler.Path(), handler.Handle()).Methods(handler.Method())

	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	require.NoError(t, err)
	require.Contains(t, rr.Body.String(), "Temporary Redirect")
	require.Equal(t, http.StatusTemporaryRedirect, rr.Code)
}

func TestOperation_Callback(t *testing.T) {
	cms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "users") {
			fmt.Fprintln(w, fmt.Sprintf("[%s]", foo))
		} else {
			fmt.Fprintln(w, jsonArray)
		}
	}))
	defer cms.Close()

	router := mux.NewRouter()
	router.HandleFunc("/store", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	router.HandleFunc("/credential", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, err := writer.Write([]byte(testCredentialRequest))
		if err != nil {
			panic(err)
		}
	})

	vcs := httptest.NewServer(router)

	defer vcs.Close()

	headers := make(map[string]string)
	headers["Authorization"] = authHeader

	file, err := ioutil.TempFile("", "*.html")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(file.Name())) }()

	cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
		CMSURL: cms.URL, VCSURL: vcs.URL, ReceiveVCHTML: file.Name(), QRCodeHTML: file.Name()}
	handler := getHandlerWithConfig(t, callback, cfg)

	_, status, err := handleRequest(handler, headers, callback, true)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// test ledger cookie not found
	cfg = &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
		CMSURL: cms.URL, VCSURL: vcs.URL, ReceiveVCHTML: file.Name()}
	handler = getHandlerWithConfig(t, callback, cfg)

	body, status, err := handleRequest(handler, headers, callback, false)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body.String(), "failed to get cookie")

	// test html not exist
	cfg = &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
		CMSURL: cms.URL, VCSURL: vcs.URL, ReceiveVCHTML: ""}
	handler = getHandlerWithConfig(t, callback, cfg)

	body, status, err = handleRequest(handler, headers, callback, true)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, status)
	require.Contains(t, body.String(), "unable to load html")
}

func TestOperation_RetrieveVC(t *testing.T) {
	headers := make(map[string]string)
	headers["Authorization"] = authHeader

	file, err := ioutil.TempFile("", "*.html")
	require.NoError(t, err)

	defer func() { require.NoError(t, os.Remove(file.Name())) }()

	t.Run("retrieve credential success", func(t *testing.T) {
		router := mux.NewRouter()

		router.HandleFunc("/retrieve", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, err := writer.Write([]byte(`"credential"`))
			if err != nil {
				panic(err)
			}
		})

		vcs := httptest.NewServer(router)

		defer vcs.Close()

		cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
			VCSURL: vcs.URL, ReceiveVCHTML: file.Name(), QRCodeHTML: file.Name()}

		handler := getHandlerWithConfig(t, retrieve, cfg)
		_, status, err := handleRequest(handler, headers, retrieve, true)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
	})
	t.Run("retrieve credential error", func(t *testing.T) {
		router := mux.NewRouter()

		router.HandleFunc("/retrieve", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusInternalServerError)
		})

		vcs := httptest.NewServer(router)

		defer vcs.Close()

		cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
			VCSURL: vcs.URL, ReceiveVCHTML: file.Name(), QRCodeHTML: file.Name()}
		handler := getHandlerWithConfig(t, retrieve, cfg)

		_, status, err := handleRequest(handler, headers, retrieve, true)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, status)
	})
}

func TestOperation_GenerateQRCode(t *testing.T) {
	t.Run("qr code success", func(t *testing.T) {
		qr, err := generateQRCode([]byte(`{"id":"test"}`), "host", "profile")
		require.NoError(t, err)
		require.NotNil(t, qr)
	})
	t.Run("qr code error", func(t *testing.T) {
		qr, err := generateQRCode([]byte(`{"name":chan int}`), "host", "profile")
		require.Error(t, err)
		require.Contains(t, err.Error(), "generate QR Code unmarshalling failed")
		require.Nil(t, qr)
	})
}

func TestOperation_RetrieveCredential(t *testing.T) {
	t.Run("retrieve credential success", func(t *testing.T) {
		vcs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "ok")
		}))
		defer vcs.Close()
		svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{}, VCSURL: vcs.URL})
		cred, err := svc.retrieveCredential("test", "")
		require.NoError(t, err)
		require.NotNil(t, cred)
	})
	t.Run("retrieve credential  error invalid url ", func(t *testing.T) {
		svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
			VCSURL: "%%&^$"})
		cred, err := svc.retrieveCredential("test", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid URL escape")
		require.Nil(t, cred)
	})
	t.Run("retrieve credential error incorrect status", func(t *testing.T) {
		vcs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, "{}")
		}))
		defer vcs.Close()
		svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{}, VCSURL: vcs.URL})
		cred, err := svc.retrieveCredential("test", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "201 Created")
		require.Nil(t, cred)
	})
}

func TestOperation_Callback_ExchangeCodeError(t *testing.T) {
	svc := New(&Config{
		TokenIssuer:   &mockTokenIssuer{err: errors.New("exchange code error")},
		TokenResolver: &mockTokenResolver{}})
	require.NotNil(t, svc)

	handler := handlerLookup(t, svc, callback)

	body, status, err := handleRequest(handler, nil, callback, true)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body.String(), "failed to exchange code for token")
	require.Contains(t, body.String(), "exchange code error")
}

func TestOperation_Callback_TokenIntrospectionError(t *testing.T) {
	headers := make(map[string]string)
	headers["Authorization"] = authHeader

	svc := New(&Config{
		TokenIssuer:   &mockTokenIssuer{},
		TokenResolver: &mockTokenResolver{err: errors.New("token info error")}})
	require.NotNil(t, svc)

	handler := handlerLookup(t, svc, callback)
	body, status, err := handleRequest(handler, headers, callback, true)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body.String(), "failed to get token info")
	require.Contains(t, body.String(), "token info error")
}

func TestOperation_Callback_GetCMSData_Error(t *testing.T) {
	headers := make(map[string]string)
	headers["Authorization"] = authHeader

	cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
		CMSURL: "cms"}
	handler := getHandlerWithConfig(t, callback, cfg)

	data, status, err := handleRequest(handler, headers, callback, true)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, data.String(), "unsupported protocol scheme")
}

func TestOperation_Callback_CreateCredential_Error(t *testing.T) {
	cms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, jsonArray)
	}))
	defer cms.Close()

	headers := make(map[string]string)
	headers["Authorization"] = authHeader

	cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
		CMSURL: cms.URL, VCSURL: "vcs"}
	handler := getHandlerWithConfig(t, callback, cfg)

	data, status, err := handleRequest(handler, headers, callback, true)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, status)
	require.Contains(t, data.String(), "unsupported protocol scheme")
}

func TestOperation_Callback_StoreCredential_Error(t *testing.T) {
	cms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, jsonArray)
	}))
	defer cms.Close()

	vcs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, "{}")
	}))

	defer vcs.Close()

	headers := make(map[string]string)
	headers["Authorization"] = authHeader

	cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
		CMSURL: cms.URL, VCSURL: vcs.URL}
	handler := getHandlerWithConfig(t, callback, cfg)

	data, status, err := handleRequest(handler, headers, callback, true)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, status)
	require.Contains(t, data.String(), "failed to store credential")
}

func TestOperation_StoreCredential(t *testing.T) {
	t.Run("store credential success", func(t *testing.T) {
		vcs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "{}")
		}))
		defer vcs.Close()
		svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{}, VCSURL: vcs.URL})
		err := svc.storeCredential([]byte(testCredentialRequest), "")
		require.NoError(t, err)
	})
	t.Run("store credential error invalid url ", func(t *testing.T) {
		svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
			VCSURL: "%%&^$"})
		err := svc.storeCredential([]byte(testCredentialRequest), "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid URL escape")
	})
	t.Run("store credential error incorrect status", func(t *testing.T) {
		vcs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, "{}")
		}))
		defer vcs.Close()
		svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{}, VCSURL: vcs.URL})
		err := svc.storeCredential([]byte(testCredentialRequest), "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "201 Created")
	})
}
func TestOperation_GetCMSData_InvalidURL(t *testing.T) {
	svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
		CMSURL: "xyz:cms"})
	require.NotNil(t, svc)

	data, err := svc.getCMSData(&oauth2.Token{}, &token.Introspection{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported protocol scheme")
	require.Nil(t, data)
}

func TestOperation_GetCMSData_InvalidHTTPRequest(t *testing.T) {
	svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
		CMSURL: "http://cms\\"})
	require.NotNil(t, svc)

	data, err := svc.getCMSData(&oauth2.Token{}, &token.Introspection{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid character")
	require.Nil(t, data)
}
func TestOperation_CreateCredential_Errors(t *testing.T) {
	cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{}}

	var subject map[string]interface{} = make(map[string]interface{})
	subject["id"] = "1"

	t.Run("unsupported protocol scheme", func(t *testing.T) {
		cfg.VCSURL = "xyz:vcs"
		svc := New(cfg)
		require.NotNil(t, svc)

		data, err := svc.createCredential(subject, &token.Introspection{}, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported protocol scheme")
		require.Nil(t, data)
	})
	t.Run("invalid http request", func(t *testing.T) {
		cfg.VCSURL = "http://vcs\\"
		svc := New(cfg)
		require.NotNil(t, svc)

		data, err := svc.createCredential(subject, &token.Introspection{}, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid character")
		require.Nil(t, data)
	})
	t.Run("invalid subject map - contains channel", func(t *testing.T) {
		subject["invalid"] = make(chan int, 2)
		svc := New(cfg)
		require.NotNil(t, svc)

		data, err := svc.createCredential(subject, &token.Introspection{}, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported type: chan int")
		require.Nil(t, data)
	})
}

func TestOperation_GetCMSUser(t *testing.T) {
	cfg := &Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{}}

	t.Run("test success", func(t *testing.T) {
		cms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, fmt.Sprintf("[%s]", foo))
		}))
		defer cms.Close()

		cfg.CMSURL = cms.URL
		svc := New(cfg)
		require.NotNil(t, svc)

		data, err := svc.getCMSData(&oauth2.Token{}, &token.Introspection{})
		require.NoError(t, err)
		require.Equal(t, data["email"], "foo@bar.com")
	})
	t.Run("no user found", func(t *testing.T) {
		cms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "[]")
		}))
		defer cms.Close()

		cfg.CMSURL = cms.URL
		svc := New(cfg)
		require.NotNil(t, svc)

		data, err := svc.getCMSData(&oauth2.Token{}, &token.Introspection{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "user not found")
		require.Nil(t, data)
	})
}

func TestOperation_UnmarshalUser(t *testing.T) {
	t.Run("test success", func(t *testing.T) {
		user, err := unmarshalUser([]byte(fmt.Sprintf("[%s]", foo)))
		require.NoError(t, err)
		require.Equal(t, user.Email, "foo@bar.com")
	})
	t.Run("json unmarshal error", func(t *testing.T) {
		data, err := unmarshalUser([]byte("invalid"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid character")
		require.Nil(t, data)
	})
	t.Run("user not found", func(t *testing.T) {
		data, err := unmarshalUser([]byte("[]"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "user not found")
		require.Nil(t, data)
	})
	t.Run("multiple users error", func(t *testing.T) {
		data, err := unmarshalUser([]byte(fmt.Sprintf("[{},{}]")))
		require.Error(t, err)
		require.Contains(t, err.Error(), "multiple users found")
		require.Nil(t, data)
	})
}

func TestOperation_UnmarshalSubject(t *testing.T) {
	t.Run("test success", func(t *testing.T) {
		data, err := unmarshalSubject([]byte(`[{"email":"foo@bar.com"}]`))
		require.NoError(t, err)
		require.Equal(t, data["email"], "foo@bar.com")
	})
	t.Run("json unmarshal error", func(t *testing.T) {
		data, err := unmarshalSubject([]byte("invalid"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid character")
		require.Nil(t, data)
	})
	t.Run("record not found", func(t *testing.T) {
		data, err := unmarshalSubject([]byte("[]"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "record not found")
		require.Nil(t, data)
	})
	t.Run("multiple records error", func(t *testing.T) {
		data, err := unmarshalSubject([]byte(fmt.Sprintf("[{},{}]")))
		require.Error(t, err)
		require.Contains(t, err.Error(), "multiple records found")
		require.Nil(t, data)
	})
}

func TestOperation_SendHTTPRequest_WrongStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "{}")
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL, nil)
	require.NoError(t, err)

	data, err := sendHTTPRequest(req, http.DefaultClient, http.StatusInternalServerError)
	require.Error(t, err)
	require.Contains(t, err.Error(), "200 OK")
	require.Nil(t, data)
}

func TestRevokeVC(t *testing.T) {
	t.Run("test error from parse form", func(t *testing.T) {
		svc := New(&Config{})

		rr := httptest.NewRecorder()
		svc.revokeVC(rr, &http.Request{Method: http.MethodPost})
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		require.Contains(t, rr.Body.String(), "failed to parse form")
	})

	t.Run("test error from create http request", func(t *testing.T) {
		svc := New(&Config{TokenIssuer: &mockTokenIssuer{}, TokenResolver: &mockTokenResolver{},
			VCSURL: "http://vcs\\"})
		require.NotNil(t, svc)

		rr := httptest.NewRecorder()
		m := make(map[string][]string)
		m["vcDataInput"] = []string{"vc"}
		svc.revokeVC(rr, &http.Request{Form: m})
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		require.Contains(t, rr.Body.String(), "failed to create new http request")
	})

	t.Run("test error from http post", func(t *testing.T) {
		svc := New(&Config{})

		rr := httptest.NewRecorder()
		m := make(map[string][]string)
		m["vcDataInput"] = []string{"vc"}
		svc.revokeVC(rr, &http.Request{Form: m})
		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Contains(t, rr.Body.String(), "failed to update vc status")
	})

	t.Run("test vc html not exist", func(t *testing.T) {
		router := mux.NewRouter()
		router.HandleFunc(vcsUpdateStatusEndpoint, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		vcs := httptest.NewServer(router)

		defer vcs.Close()

		svc := New(&Config{VCHTML: "", VCSURL: vcs.URL})

		rr := httptest.NewRecorder()
		m := make(map[string][]string)
		m["vcDataInput"] = []string{"vc"}
		svc.revokeVC(rr, &http.Request{Form: m})
		require.Equal(t, http.StatusInternalServerError, rr.Code)
		require.Contains(t, rr.Body.String(), "unable to load html")
	})

	t.Run("test success", func(t *testing.T) {
		file, err := ioutil.TempFile("", "*.html")
		require.NoError(t, err)

		defer func() { require.NoError(t, os.Remove(file.Name())) }()
		router := mux.NewRouter()
		router.HandleFunc(vcsUpdateStatusEndpoint, func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		vcs := httptest.NewServer(router)

		defer vcs.Close()

		svc := New(&Config{VCHTML: file.Name(), VCSURL: vcs.URL})

		rr := httptest.NewRecorder()
		m := make(map[string][]string)
		m["vcDataInput"] = []string{"vc"}

		svc.revokeVC(rr, &http.Request{Form: m})
		require.Equal(t, http.StatusOK, rr.Code)
	})
}

func handleRequest(handler Handler, headers map[string]string, path string, addCookie bool) (*bytes.Buffer, int, error) { //nolint:lll
	req, err := http.NewRequest(handler.Method(), path, bytes.NewBuffer([]byte("")))
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	if addCookie {
		req.AddCookie(&http.Cookie{Name: vcsProfileCookie, Value: "vc-issuer-1"})
	}

	router := mux.NewRouter()

	router.HandleFunc(handler.Path(), handler.Handle()).Methods(handler.Method())

	// create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	return rr.Body, rr.Code, nil
}

func getHandlerWithConfig(t *testing.T, lookup string, cfg *Config) Handler {
	svc := New(cfg)
	require.NotNil(t, svc)

	return handlerLookup(t, svc, lookup)
}

func handlerLookup(t *testing.T, op *Operation, lookup string) Handler {
	handlers := op.GetRESTHandlers()
	require.NotEmpty(t, handlers)

	for _, h := range handlers {
		if h.Path() == lookup {
			return h
		}
	}

	require.Fail(t, "unable to find handler")

	return nil
}

type mockTokenIssuer struct {
	err error
}

func (m *mockTokenIssuer) AuthCodeURL(w http.ResponseWriter) string {
	return "url"
}

func (m *mockTokenIssuer) Exchange(r *http.Request) (*oauth2.Token, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &oauth2.Token{}, nil
}

func (m *mockTokenIssuer) Client(t *oauth2.Token) *http.Client {
	return http.DefaultClient
}

type mockTokenResolver struct {
	err error
}

func (r *mockTokenResolver) Resolve(tk string) (*token.Introspection, error) {
	if r.err != nil {
		return nil, r.err
	}

	return &token.Introspection{}, nil
}
