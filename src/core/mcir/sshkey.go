/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mcir is to manage multi-cloud infra resource
package mcir

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
)

// SpiderKeyPairReqInfoWrapper is a wrapper struct to create JSON body of 'Create keypair request'
type SpiderKeyPairReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderKeyPairInfo
}

// SpiderKeyPairInfo is a struct to create JSON body of 'Create keypair request'
type SpiderKeyPairInfo struct {
	// Fields for request
	Name  string
	CSPId string

	// Fields for response
	IId          common.IID // {NameId, SystemId}
	Fingerprint  string
	PublicKey    string
	PrivateKey   string
	VMUserID     string
	KeyValueList []common.KeyValue
}

// TbSshKeyReq is a struct to handle 'Create SSH key' request toward CB-Tumblebug.
type TbSshKeyReq struct {
	Name           string `json:"name" validate:"required"`
	ConnectionName string `json:"connectionName" validate:"required"`
	Description    string `json:"description"`

	// Fields for "Register existing SSH keys" feature
	// CspSshKeyId is required to register object from CSP (option=register)
	CspSshKeyId      string `json:"cspSshKeyId"`
	Fingerprint      string `json:"fingerprint"`
	Username         string `json:"username"`
	VerifiedUsername string `json:"verifiedUsername"`
	PublicKey        string `json:"publicKey"`
	PrivateKey       string `json:"privateKey"`
}

// TbSshKeyReqStructLevelValidation is a function to validate 'TbSshKeyReq' object.
func TbSshKeyReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbSshKeyReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

// TbSshKeyInfo is a struct that represents TB SSH key object.
type TbSshKeyInfo struct {
	Id             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	ConnectionName string `json:"connectionName,omitempty"`
	Description    string `json:"description,omitempty"`

	// CspSshKeyId used for CSP-native identifier (either Name or ID)
	CspSshKeyId string `json:"cspSshKeyId,omitempty"`

	// CspSshKeyName used for CB-Spider identifier
	CspSshKeyName string `json:"cspSshKeyName,omitempty"`

	Fingerprint          string            `json:"fingerprint,omitempty"`
	Username             string            `json:"username,omitempty"`
	VerifiedUsername     string            `json:"verifiedUsername,omitempty"`
	PublicKey            string            `json:"publicKey,omitempty"`
	PrivateKey           string            `json:"privateKey,omitempty"`
	KeyValueList         []common.KeyValue `json:"keyValueList,omitempty"`
	AssociatedObjectList []string          `json:"associatedObjectList,omitempty"`
	IsAutoGenerated      bool              `json:"isAutoGenerated,omitempty"`

	// SystemLabel is for describing the MCIR in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel,omitempty" example:"Managed by CB-Tumblebug" default:""`
}

// CreateSshKey accepts SSH key creation request, creates and returns an TB sshKey object
func CreateSshKey(nsId string, u *TbSshKeyReq, option string) (TbSshKeyInfo, error) {

	resourceType := common.StrSSHKey

	err := common.CheckString(nsId)
	if err != nil {
		temp := TbSshKeyInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	if option == "register" { // fields validation
		errs := []error{}
		// errs = append(errs, validate.Var(u.Username, "required"))
		// errs = append(errs, validate.Var(u.PrivateKey, "required"))

		for _, err := range errs {
			if err != nil {
				temp := TbSshKeyInfo{}
				if _, ok := err.(*validator.InvalidValidationError); ok {
					fmt.Println(err)
					return temp, err
				}
				return temp, err
			}
		}
	}

	err = validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			temp := TbSshKeyInfo{}
			return temp, err
		}

		temp := TbSshKeyInfo{}
		return temp, err
	}

	check, err := CheckResource(nsId, resourceType, u.Name)

	if check {
		temp := TbSshKeyInfo{}
		err := fmt.Errorf("The sshKey %s already exists.", u.Name)
		return temp, err
	}

	if err != nil {
		temp := TbSshKeyInfo{}
		err := fmt.Errorf("Failed to check the existence of the sshKey %s.", u.Name)
		return temp, err
	}

	tempReq := SpiderKeyPairReqInfoWrapper{}
	tempReq.ConnectionName = u.ConnectionName
	tempReq.ReqInfo.Name = fmt.Sprintf("%s-%s", nsId, u.Name)
	tempReq.ReqInfo.CSPId = u.CspSshKeyId

	var tempSpiderKeyPairInfo *SpiderKeyPairInfo

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		client := resty.New().SetCloseConnection(true)
		client.SetAllowGetMethodPayload(true)

		req := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq).
			SetResult(&SpiderKeyPairInfo{}) // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).

		var resp *resty.Response
		var err error

		var url string
		if option == "register" && u.CspSshKeyId == "" {
			url = fmt.Sprintf("%s/keypair/%s", common.SpiderRestUrl, u.Name)
			resp, err = req.Get(url)
		} else if option == "register" && u.CspSshKeyId != "" {
			url = fmt.Sprintf("%s/regkeypair", common.SpiderRestUrl)
			resp, err = req.Post(url)
		} else { // option != "register"
			url = fmt.Sprintf("%s/keypair", common.SpiderRestUrl)
			resp, err = req.Post(url)
		}

		if err != nil {
			common.CBLog.Error(err)
			content := TbSshKeyInfo{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Printf("HTTP Status code: %d \n", resp.StatusCode())
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			fmt.Println("body: ", string(resp.Body()))
			common.CBLog.Error(err)
			content := TbSshKeyInfo{}
			return content, err
		}

		tempSpiderKeyPairInfo = resp.Result().(*SpiderKeyPairInfo)

	} else { // gRPC

		// Set CCM gRPC API
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return TbSshKeyInfo{}, err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return TbSshKeyInfo{}, err
		}
		defer ccm.Close()

		payload, _ := json.MarshalIndent(tempReq, "", "  ")

		result, err := ccm.CreateKey(string(payload))
		if err != nil {
			common.CBLog.Error(err)
			return TbSshKeyInfo{}, err
		}

		tempSpiderKeyPairInfo = &SpiderKeyPairInfo{}
		err = json.Unmarshal([]byte(result), &tempSpiderKeyPairInfo)
		if err != nil {
			common.CBLog.Error(err)
			return TbSshKeyInfo{}, err
		}

	}

	content := TbSshKeyInfo{}
	//content.Id = common.GenUid()
	content.Id = u.Name
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	fmt.Printf("tempSpiderKeyPairInfo.IId.SystemId: %s \n", tempSpiderKeyPairInfo.IId.SystemId)
	content.CspSshKeyId = tempSpiderKeyPairInfo.IId.SystemId
	content.CspSshKeyName = tempSpiderKeyPairInfo.IId.NameId
	content.Fingerprint = tempSpiderKeyPairInfo.Fingerprint
	content.Username = tempSpiderKeyPairInfo.VMUserID
	content.PublicKey = tempSpiderKeyPairInfo.PublicKey
	content.PrivateKey = tempSpiderKeyPairInfo.PrivateKey
	content.Description = u.Description
	content.KeyValueList = tempSpiderKeyPairInfo.KeyValueList
	content.AssociatedObjectList = []string{}

	if option == "register" {
		if u.CspSshKeyId == "" {
			content.SystemLabel = "Registered from CB-Spider resource"
		} else if u.CspSshKeyId != "" {
			content.SystemLabel = "Registered from CSP resource"
		}

		// Rewrite fields again
		// content.Fingerprint = u.Fingerprint
		content.Username = u.Username
		content.PublicKey = u.PublicKey
		content.PrivateKey = u.PrivateKey
	}

	// cb-store
	fmt.Println("=========================== PUT CreateSshKey")
	Key := common.GenResourceKey(nsId, resourceType, content.Id)
	Val, _ := json.Marshal(content)
	err = common.CBStore.Put(Key, string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}
	return content, nil
}

// UpdateSshKey accepts to-be TB sshKey objects,
// updates and returns the updated TB sshKey objects
func UpdateSshKey(nsId string, sshKeyId string, fieldsToUpdate TbSshKeyInfo) (TbSshKeyInfo, error) {
	resourceType := common.StrSSHKey

	err := common.CheckString(nsId)
	if err != nil {
		temp := TbSshKeyInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	if len(fieldsToUpdate.Id) > 0 {
		temp := TbSshKeyInfo{}
		err := fmt.Errorf("You should not specify 'id' in the JSON request body.")
		common.CBLog.Error(err)
		return temp, err
	}

	check, err := CheckResource(nsId, resourceType, sshKeyId)

	if err != nil {
		temp := TbSshKeyInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	if !check {
		temp := TbSshKeyInfo{}
		err := fmt.Errorf("The sshKey %s does not exist.", sshKeyId)
		return temp, err
	}

	tempInterface, err := GetResource(nsId, resourceType, sshKeyId)
	if err != nil {
		temp := TbSshKeyInfo{}
		err := fmt.Errorf("Failed to get the sshKey %s.", sshKeyId)
		return temp, err
	}
	asIsSshKey := TbSshKeyInfo{}
	err = common.CopySrcToDest(&tempInterface, &asIsSshKey)
	if err != nil {
		temp := TbSshKeyInfo{}
		err := fmt.Errorf("Failed to CopySrcToDest() %s.", sshKeyId)
		return temp, err
	}

	// Update specified fields only
	toBeSshKey := asIsSshKey
	toBeSshKeyJSON, _ := json.Marshal(fieldsToUpdate)
	err = json.Unmarshal(toBeSshKeyJSON, &toBeSshKey)

	// cb-store
	fmt.Println("=========================== PUT UpdateSshKey")
	Key := common.GenResourceKey(nsId, resourceType, toBeSshKey.Id)
	Val, _ := json.Marshal(toBeSshKey)
	err = common.CBStore.Put(Key, string(Val))
	if err != nil {
		temp := TbSshKeyInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	keyValue, err := common.CBStore.Get(Key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In UpdateSshKey(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	fmt.Printf("<%s> \n %s \n", keyValue.Key, keyValue.Value)
	fmt.Println("===========================")

	_, err = common.ORM.Update(&toBeSshKey, &TbSshKeyInfo{Id: sshKeyId})
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("SQL data updated successfully..")
	}

	return toBeSshKey, nil
}
