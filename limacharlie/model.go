package limacharlie

import (
	"fmt"
	"net/http"
)

type Model struct {
	modelName string
	org       *Organization
}

func NewModel(org *Organization, modelName string) *Model {
	return &Model{
		modelName: modelName,
		org:       org,
	}
}

func (m *Model) Mget(indexKeyName, indexKeyValue string) (interface{}, error) {
	var resp interface{}
	request := makeDefaultRequest(&resp).withQueryData(Dict{
		"model_name":      m.modelName,
		"index_key_name":  indexKeyName,
		"index_key_value": indexKeyValue,
	})
	err := m.org.client.reliableRequest(http.MethodGet, fmt.Sprintf("models/%s/model/%s/records", m.org.GetOID(), m.modelName), request)
	return resp, err
}

func (m *Model) Get(primaryKey string) (interface{}, error) {
	var resp interface{}
	request := makeDefaultRequest(&resp).withQueryData(Dict{
		"primary_key": primaryKey,
	})
	err := m.org.client.reliableRequest(http.MethodGet, fmt.Sprintf("models/%s/model/%s/record", m.org.GetOID(), m.modelName), request)
	return resp, err
}

func (m *Model) Delete(primaryKey string) (interface{}, error) {
	var resp interface{}
	request := makeDefaultRequest(&resp).withQueryData(Dict{
		"primary_key": primaryKey,
	})
	err := m.org.client.reliableRequest(http.MethodDelete, fmt.Sprintf("models/%s/model/%s/record", m.org.GetOID(), m.modelName), request)
	return resp, err
}

func (m *Model) Query(startIndexKeyName, startIndexKeyValue string, plan []interface{}) (interface{}, error) {
	var resp interface{}
	request := makeDefaultRequest(&resp).withQueryData(Dict{
		"starting_model_name": m.modelName,
		"starting_key_name":   startIndexKeyName,
		"starting_key_value":  startIndexKeyValue,
	})
	err := m.org.client.reliableRequest(http.MethodGet, fmt.Sprintf("models/%s/query", m.org.GetOID()), request)
	return resp, err
}

func (m *Model) Add(primaryKey string, fields map[string]interface{}) (interface{}, error) {
	var resp interface{}
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	request := makeDefaultRequest(&resp).withQueryData(Dict{
		"model_name":  m.modelName,
		"primary_key": primaryKey,
		"fields":      string(fieldsJSON),
	})
	err = m.org.client.reliableRequest(http.MethodPost, fmt.Sprintf("models/%s/model/%s/record", m.org.GetOID(), m.modelName), request)
	return resp, err
}
