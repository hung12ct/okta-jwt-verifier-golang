/*******************************************************************************
 * Copyright 2018 - Present Okta, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 ******************************************************************************/

package jwtverifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hung12ct/okta-jwt-verifier-golang/v2/adaptors/lestrratGoJwx"
	"github.com/hung12ct/okta-jwt-verifier-golang/v2/discovery/oidc"
	"github.com/hung12ct/okta-jwt-verifier-golang/v2/utils"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func Test_the_verifier_defaults_to_oidc_if_nothing_is_provided_for_discovery(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "issuer",
	}

	jv, _ := jvs.New()

	if reflect.TypeOf(jv.GetDiscovery()) != reflect.TypeOf(oidc.Oidc{}) {
		t.Errorf("discovery did not set to oidc by default.  Was set to: %s",
			reflect.TypeOf(jv.GetDiscovery()))
	}
}

func Test_the_verifier_defaults_to_lestrratGoJwx_if_nothing_is_provided_for_adaptor(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "issuer",
	}

	jv, _ := jvs.New()

	if reflect.TypeOf(jv.GetAdaptor()) != reflect.TypeOf(&lestrratGoJwx.LestrratGoJwx{}) {
		t.Errorf("adaptor did not set to lestrratGoJwx by default.  Was set to: %s",
			reflect.TypeOf(jv.GetAdaptor()))
	}
}

func Test_can_validate_iss_from_issuer_provided(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	err := jv.validateIss("test")
	if err == nil {
		t.Errorf("the issuer validation did not trigger an error")
	}
}

func Test_can_validate_nonce(t *testing.T) {
	tv := map[string]string{}
	tv["nonce"] = "abc123"

	jvs := JwtVerifier{
		Issuer:           "https://golang.oktapreview.com",
		ClaimsToValidate: tv,
	}

	jv, _ := jvs.New()

	err := jv.validateNonce("test")
	if err == nil {
		t.Errorf("the nonce validation did not trigger an error")
	}
}

func Test_can_validate_aud(t *testing.T) {
	tv := map[string]string{}
	tv["aud"] = "abc123"

	jvs := JwtVerifier{
		Issuer:           "https://golang.oktapreview.com",
		ClaimsToValidate: tv,
	}

	jv, _ := jvs.New()

	err := jv.validateAudience("test")
	if err == nil {
		t.Errorf("the audience validation did not trigger an error")
	}
}

func Test_can_validate_cid(t *testing.T) {
	tv := map[string]string{}
	tv["cid"] = "abc123"

	jvs := JwtVerifier{
		Issuer:           "https://golang.oktapreview.com",
		ClaimsToValidate: tv,
	}

	jv, _ := jvs.New()

	err := jv.validateClientId("test")
	if err == nil {
		t.Errorf("the cid validation did not trigger an error")
	}
}

func Test_can_validate_iat(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	// token issued in future triggers error
	err := jv.validateIat(float64(time.Now().Unix() + 300))
	if err == nil {
		t.Errorf("the iat validation did not trigger an error")
	}

	// token within leeway does not trigger error
	err = jv.validateIat(float64(time.Now().Unix()))
	if err != nil {
		t.Errorf("the iat validation triggered an error")
	}
}

func Test_can_validate_exp(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	// expired token triggers error
	err := jv.validateExp(float64(time.Now().Unix() - 300))
	if err == nil {
		t.Errorf("the exp validation did not trigger an error for expired token")
	}

	// token within leeway does not trigger error
	err = jv.validateExp(float64(time.Now().Unix()))
	if err != nil {
		t.Errorf("the exp validation triggered an error for valid token")
	}
}

// ID TOKEN TESTS
func Test_invalid_formatting_of_id_token_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyIdToken("aa")

	if err == nil {
		t.Errorf("an error was not thrown when an id token does not contain at least 1 period ('.')")
	}

	if !strings.Contains(err.Error(), "token must contain at least 1 period ('.')") {
		t.Errorf("the error for id token with no periods did not trigger")
	}
}

func Test_an_id_token_header_that_is_improperly_formatted_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyIdToken("123456789.aa.aa")

	if !strings.Contains(err.Error(), "does not appear to be a base64 encoded string") {
		t.Errorf("the error for id token with header that is not base64 encoded did not trigger")
	}
}

func Test_an_id_token_header_that_is_not_decoded_into_json_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyIdToken("aa.aa.aa")

	if !strings.Contains(err.Error(), "not a json object") {
		t.Errorf("the error for id token with header that is not a json object did not trigger")
	}
}

func Test_an_id_token_header_that_is_not_contain_the_correct_parts_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyIdToken("ew0KICAia2lkIjogImFiYzEyMyIsDQogICJhbmQiOiAidGhpcyINCn0.aa.aa")

	if !strings.Contains(err.Error(), "header must contain an 'alg'") {
		t.Errorf("the error for id token with header that did not contain alg did not trigger")
	}

	_, err = jv.VerifyIdToken("ew0KICAiYWxnIjogIlJTMjU2IiwNCiAgImFuZCI6ICJ0aGlzIg0KfQ.aa.aa")

	if !strings.Contains(err.Error(), "header must contain a 'kid'") {
		t.Errorf("the error for id token with header that did not contain kid did not trigger")
	}
}

func Test_an_id_token_header_that_is_not_rs256_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyIdToken("ew0KICAia2lkIjogImFiYzEyMyIsDQogICJhbGciOiAiSFMyNTYiDQp9.aa.aa")

	if !strings.Contains(err.Error(), "only supported alg is RS256") {
		t.Errorf("the error for id token with with wrong alg did not trigger")
	}
}

// ACCESS TOKEN TESTS
func Test_invalid_formatting_of_access_token_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyAccessToken("aa")

	if err == nil {
		t.Errorf("an error was not thrown when an access token does not contain at least 1 period ('.')")
	}

	if !strings.Contains(err.Error(), "token must contain at least 1 period ('.')") {
		t.Errorf("the error for access token with no periods did not trigger")
	}
}

func Test_an_access_token_header_that_is_improperly_formatted_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyAccessToken("123456789.aa.aa")

	if !strings.Contains(err.Error(), "does not appear to be a base64 encoded string") {
		t.Errorf("the error for access token with header that is not base64 encoded did not trigger")
	}
}

func Test_an_access_token_header_that_is_not_decoded_into_json_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyAccessToken("aa.aa.aa")

	if !strings.Contains(err.Error(), "not a json object") {
		t.Errorf("the error for access token with header that is not a json object did not trigger")
	}
}

func Test_an_access_token_header_that_is_not_contain_the_correct_parts_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyAccessToken("ew0KICAia2lkIjogImFiYzEyMyIsDQogICJhbmQiOiAidGhpcyINCn0.aa.aa")

	if !strings.Contains(err.Error(), "header must contain an 'alg'") {
		t.Errorf("the error for access token with header that did not contain alg did not trigger")
	}

	_, err = jv.VerifyAccessToken("ew0KICAiYWxnIjogIlJTMjU2IiwNCiAgImFuZCI6ICJ0aGlzIg0KfQ.aa.aa")

	if !strings.Contains(err.Error(), "header must contain a 'kid'") {
		t.Errorf("the error for access token with header that did not contain kid did not trigger")
	}
}

func Test_an_access_token_header_that_is_not_rs256_throws_an_error(t *testing.T) {
	jvs := JwtVerifier{
		Issuer: "https://golang.oktapreview.com",
	}

	jv, _ := jvs.New()

	_, err := jv.VerifyAccessToken("ew0KICAia2lkIjogImFiYzEyMyIsDQogICJhbGciOiAiSFMyNTYiDQp9.aa.aa")

	if !strings.Contains(err.Error(), "only supported alg is RS256") {
		t.Errorf("the error for access token with with wrong alg did not trigger")
	}
}

func Test_a_successful_authentication_can_have_its_tokens_parsed(t *testing.T) {
	utils.ParseEnvironment()

	if os.Getenv("ISSUER") == "" || os.Getenv("CLIENT_ID") == "" {
		log.Printf("Skipping integration tests")
		t.Skip("appears that environment variables are not set, skipping the integration test for now")
	}

	type AuthnResponse struct {
		SessionToken string `json:"sessionToken"`
	}

	nonce, err := utils.GenerateNonce()
	if err != nil {
		t.Errorf("could not generate nonce")
	}

	// Get Session Token
	issuerParts, _ := url.Parse(os.Getenv("ISSUER"))
	baseUrl := issuerParts.Scheme + "://" + issuerParts.Hostname()
	requestUri := baseUrl + "/api/v1/authn"
	postValues := map[string]string{"username": os.Getenv("USERNAME"), "password": os.Getenv("PASSWORD")}
	postJsonValues, _ := json.Marshal(postValues)
	resp, err := http.Post(requestUri, "application/json", bytes.NewReader(postJsonValues))
	if err != nil {
		t.Errorf("could not submit authentication endpoint")
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, resp.Body)

	var authn AuthnResponse
	err = json.Unmarshal(buf.Bytes(), &authn)
	if err != nil {
		t.Errorf("could not unmarshal authn response")
	}

	// Issue get request with session token to get id/access tokens
	authzUri := os.Getenv("ISSUER") + "/v1/authorize?client_id=" + os.Getenv(
		"CLIENT_ID") + "&nonce=" + nonce + "&redirect_uri=http://localhost:8080/implicit/callback" +
		"&response_type=token%20id_token&scope=openid&state" +
		"=ApplicationState&sessionToken=" + authn.SessionToken

	client := &http.Client{
		CheckRedirect: func(req *http.Request, with []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err = client.Get(authzUri)

	if err != nil {
		t.Errorf("could not submit authorization endpoint: %s", err.Error())
	}

	defer resp.Body.Close()
	location := resp.Header.Get("Location")
	locParts, _ := url.Parse(location)
	fragmentParts, _ := url.ParseQuery(locParts.Fragment)

	if fragmentParts["access_token"] == nil {
		t.Errorf("could not extract access_token")
	}

	if fragmentParts["id_token"] == nil {
		t.Errorf("could not extract id_token")
	}

	accessToken := fragmentParts["access_token"][0]
	idToken := fragmentParts["id_token"][0]

	tv := map[string]string{}
	tv["aud"] = os.Getenv("CLIENT_ID")
	tv["nonce"] = nonce
	jv := JwtVerifier{
		Issuer:           os.Getenv("ISSUER"),
		ClaimsToValidate: tv,
	}

	jwtv1, err := jv.New()
	if err != nil {
		fmt.Println(err)
	}

	claims, err := jwtv1.VerifyIdToken(idToken)
	if err != nil {
		t.Errorf("could not verify id_token: %s", err.Error())
	}

	issuer := claims.Claims["iss"]

	if issuer == nil {
		t.Errorf("issuer claim could not be pulled from access_token")
	}

	tv = map[string]string{}
	tv["aud"] = "api://default"
	tv["cid"] = os.Getenv("CLIENT_ID")
	jv = JwtVerifier{
		Issuer:           os.Getenv("ISSUER"),
		ClaimsToValidate: tv,
	}

	jwtv2, err := jv.New()
	if err != nil {
		fmt.Println(err)
	}
	claims, err = jwtv2.VerifyAccessToken(accessToken)

	if err != nil {
		t.Errorf("could not verify access_token: %s", err.Error())
	}

	issuer = claims.Claims["iss"]

	if issuer == nil {
		t.Errorf("issuer claim could not be pulled from access_token")
	}

	// Should validate without CID
	tv = map[string]string{}
	tv["aud"] = "api://default"
	jv = JwtVerifier{
		Issuer:           "https://golang-sdk-oie.oktapreview.com/oauth2/default",
		ClaimsToValidate: tv,
	}

	jwtv3, err := jv.New()
	if err != nil {
		fmt.Println(err)
	}
	claims, err = jwtv3.VerifyAccessToken(accessToken)

	if err != nil {
		t.Errorf("could not verify access_token: %s", err.Error())
	}

	issuer = claims.Claims["iss"]

	if issuer == nil {
		t.Errorf("issuer claim could not be pulled from access_token")
	}
}

func TestWhenFetchMetaDataHas404(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	errJson := `{"errorCode":"E0000022","errorSummary":"The endpoint does not support the provided HTTP method","errorLink":"E0000022","errorId":"oaebpimEDg8TSuQwXXT-wjzwA","errorCauses":[]}`
	responder := httpmock.NewStringResponder(404, errJson)
	issuer := `https://example.com/.well-known/openid-configuration`
	httpmock.RegisterResponder("GET", issuer, responder)

	jvs := JwtVerifier{
		Issuer: "https://example.com",
	}
	jv, _ := jvs.New()
	token := `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6Im15b3JnIn0.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.ORhY_syF7eW3e4-h2Lt0i2-7yWSr3GFu4XdHtsNQTquvnrVLN2VhM6gDhoaVtZutuVpDQD-Srd6haKtQTEffrUl2IM6erWVPKNlG_ljdm2hDQ4cw58hs9CJkTkPte4RAtFwsq-zLebdk_eF__rMYqwfgkgKK_13FoG0u8nEVtSoK_2gYBPrdFONC08Uwwre_iUz1MTHugWNcITT3u866UHeNHnRARAIn5L-rKMiEH6sQyhDoGqLyfL5xpn6d1xkxtEgqvoj7F-L4Cw87i4Jzmxl8Eo3xseBe0EGU0s-zMOzqWWVBrcG_pxA9IakgNPHGiRmoQk_rc3796FuwAkYZOA`
	_, err := jv.VerifyIdToken(token)

	require.ErrorContains(t, err, "request for metadata \"https://example.com/.well-known/openid-configuration\" was not HTTP 2xx OK, it was: 404")
}

func validate(verifier *JwtVerifier, token string) {
	_, err := verifier.VerifyAccessToken(token)
	if err != nil {
		log.Printf("token not valid: %v", err)
	} else {
		log.Println("valid")
	}
}

func TestRaceCondition(t *testing.T) {
	t.Skip("Run locally to test for race condition")
	type AuthnResponse struct {
		SessionToken string `json:"sessionToken"`
	}

	nonce, err := utils.GenerateNonce()
	if err != nil {
		t.Errorf("could not generate nonce")
	}

	// Get Session Token
	issuerParts, _ := url.Parse(os.Getenv("ISSUER"))
	baseUrl := issuerParts.Scheme + "://" + issuerParts.Hostname()
	requestUri := baseUrl + "/api/v1/authn"
	postValues := map[string]string{"username": os.Getenv("USERNAME"), "password": os.Getenv("PASSWORD")}
	postJsonValues, _ := json.Marshal(postValues)
	resp, err := http.Post(requestUri, "application/json", bytes.NewReader(postJsonValues))
	if err != nil {
		t.Errorf("could not submit authentication endpoint")
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, resp.Body)

	var authn AuthnResponse
	err = json.Unmarshal(buf.Bytes(), &authn)
	if err != nil {
		t.Errorf("could not unmarshal authn response")
	}

	// Issue get request with session token to get id/access tokens
	authzUri := os.Getenv("ISSUER") + "/v1/authorize?client_id=" + os.Getenv(
		"CLIENT_ID") + "&nonce=" + nonce + "&redirect_uri=http://localhost:8080/implicit/callback" +
		"&response_type=token%20id_token&scope=openid&state" +
		"=ApplicationState&sessionToken=" + authn.SessionToken

	client := &http.Client{
		CheckRedirect: func(req *http.Request, with []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err = client.Get(authzUri)

	if err != nil {
		t.Errorf("could not submit authorization endpoint: %s", err.Error())
	}

	defer resp.Body.Close()
	location := resp.Header.Get("Location")
	locParts, _ := url.Parse(location)
	fragmentParts, _ := url.ParseQuery(locParts.Fragment)

	if fragmentParts["access_token"] == nil {
		t.Errorf("could not extract access_token")
	}

	if fragmentParts["id_token"] == nil {
		t.Errorf("could not extract id_token")
	}

	accessToken := fragmentParts["access_token"][0]
	idToken := fragmentParts["id_token"][0]

	tv := map[string]string{}
	tv["aud"] = os.Getenv("CLIENT_ID")
	tv["nonce"] = nonce
	jv := JwtVerifier{
		Issuer:           os.Getenv("ISSUER"),
		ClaimsToValidate: tv,
	}

	jwtv1, err := jv.New()
	if err != nil {
		fmt.Println(err)
	}

	claims, err := jwtv1.VerifyIdToken(idToken)
	if err != nil {
		t.Errorf("could not verify id_token: %s", err.Error())
	}

	issuer := claims.Claims["iss"]

	if issuer == nil {
		t.Errorf("issuer claim could not be pulled from access_token")
	}

	tv = map[string]string{}
	tv["aud"] = "api://default"
	tv["cid"] = os.Getenv("CLIENT_ID")
	jv = JwtVerifier{
		Issuer:           os.Getenv("ISSUER"),
		ClaimsToValidate: tv,
	}

	verifier, err := jv.New()
	if err != nil {
		fmt.Println(err)
	}

	go validate(verifier, accessToken)
	validate(verifier, accessToken)
	time.Sleep(2 * time.Second)
}
