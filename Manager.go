package limacharlie

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type ResponseJWT struct {
	jwt string `json:"jwt"`
}

type SessionConfig struct {
	userID         *string
	organizationID *string
	urlAPIJwt      string
}

type Session struct {
	Config       SessionConfig
	secretAPIKey *string
	jwt          *string
}

func (session *Session) doCall(apiCall apiCall) error {
	if !apiCall.IsNoAuth && session.jwt == nil {
		if err := session.refreshToken(); err != nil {
			return err
		}
	}

	for i := 0; i < apiCall.MaxTotalRetries; i++ {
		// apiCall.res
	}

	return nil
}

func (session *Session) refreshToken() error {
	if session.secretAPIKey != nil {
		if err := session.refreshJWT(); err != nil {
			return err
		}
	}
	return nil
}

func (session *Session) refreshJWT() error {
	if session.secretAPIKey == nil {
		return errors.New("No API key is set")
	}
	jsonData := map[string]string{"secret": *session.secretAPIKey}

	var uid = ""
	if session.Config.userID != nil {
		uid = *session.Config.userID
		jsonData["uid"] = uid
	}
	var oid = ""
	if session.Config.organizationID != nil {
		oid = *session.Config.organizationID
		jsonData["oid"] = oid
	}

	jsonValue, _ := json.Marshal(jsonData)
	response, err := http.Post(session.Config.urlAPIJwt, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return fmt.Errorf("Failed to get JWT from API key (oid=%s)(uid=%s): %s", oid, uid, err)
	}
	defer response.Body.Close()
	var responseJWT ResponseJWT
	if err := json.NewDecoder(response.Body).Decode(&responseJWT); err != nil {
		return fmt.Errorf("Failed to parse JWT response: %s", err)
	}
	*session.jwt = responseJWT.jwt
	return nil
}

type Permission interface{}
type Permissions []Permission

type Manager struct {
	session Session
}

func (manager *Manager) TestAuth(permissions Permissions) bool {
	if err := manager.session.refreshToken(); err != nil {
		return false
	}

	return false
}

type WhoAmIJsonResponse struct {
	UserPermissions *map[string]string `json:"user_perms"`
	Organizations   *[]string          `json:"orgs"`
	Permissions     *[]string          `json:"perms"`
}

func (manager *Manager) WhoAmI() (WhoAmIJsonResponse, error) {
	contentType := "application/javascript"
	rootURL := "https://api.limacharlie.io"
	version := 1
	call := apiCall{
		"GET",
		"who",
		false,
		3,
		*manager.session.jwt,
		version,
		rootURL,
		mapString{},
		&contentType,
		apiCallBodyEmpty{}}
	apiResponse := call.restCall()
	if apiResponse.err != nil {
		return WhoAmIJsonResponse{}, fmt.Errorf("Failed to execute WhoAmI: %s", apiResponse.err)
	}
	var parsedResponse WhoAmIJsonResponse
	if err := json.Unmarshal(apiResponse.result, &parsedResponse); err != nil {
		return WhoAmIJsonResponse{}, fmt.Errorf("Failed to parse json for WhoAmI: %s", err)
	}
	return parsedResponse, nil
}
