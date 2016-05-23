/*
   Copyright (c) 2016 VMware, Inc. All Rights Reserved.
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

package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/vmware/harbor/api"
	"github.com/vmware/harbor/dao"
	"github.com/vmware/harbor/models"
	"github.com/vmware/harbor/utils/log"
	registry_util "github.com/vmware/harbor/utils/registry"
	"github.com/vmware/harbor/utils/registry/auth"
)

// TargetAPI handles request to /api/targets/ping /api/targets/{}
type TargetAPI struct {
	api.BaseAPI
}

// Prepare validates the user
func (t *TargetAPI) Prepare() {
	userID := t.ValidateUser()
	isSysAdmin, err := dao.IsAdminRole(userID)
	if err != nil {
		log.Errorf("error occurred in IsAdminRole: %v", err)
		t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	if !isSysAdmin {
		t.CustomAbort(http.StatusForbidden, http.StatusText(http.StatusForbidden))
	}
}

// Ping validates whether the target is reachable and whether the credential is valid
func (t *TargetAPI) Ping() {
	var endpoint, username, password string

	idStr := t.GetString("id")
	if len(idStr) != 0 {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			t.CustomAbort(http.StatusBadRequest, fmt.Sprintf("id %s is invalid", idStr))
		}

		target, err := dao.GetRepTarget(id)
		if err != nil {
			log.Errorf("failed to get target %d: %v", id, err)
			t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
		endpoint = target.URL
		username = target.Username
		password = target.Password
	} else {
		endpoint = t.GetString("endpoint")
		if len(endpoint) == 0 {
			t.CustomAbort(http.StatusBadRequest, "id or endpoint is needed")
		}

		username = t.GetString("username")
		password = t.GetString("password")
	}

	credential := auth.NewBasicAuthCredential(username, password)
	registry, err := registry_util.NewRegistryWithCredential(endpoint, credential)
	if err != nil {
		log.Errorf("failed to create registry client: %v", err)
		t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	if err = registry.Ping(); err != nil {
		log.Errorf("failed to ping registry %s: %v", registry.Endpoint.String(), err)

		// timeout, dns resolve error, connection refused, etc.
		if urlErr, ok := err.(*url.Error); ok {
			t.CustomAbort(http.StatusBadRequest, urlErr.Error())
		}

		if regErr, ok := err.(*registry_util.Error); ok {
			t.CustomAbort(regErr.StatusCode, regErr.Detail)
		}

		t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}
}

// Head ...
func (t *TargetAPI) Head() {
	id := t.getIDFromURL()
	if id == 0 {
		t.CustomAbort(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	target, err := dao.GetRepTarget(id)
	if err != nil {
		log.Errorf("failed to get target %d: %v", id, err)
		t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	// not exist
	if target == nil {
		t.CustomAbort(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	}
}

// Get ...
func (t *TargetAPI) Get() {
	id := t.getIDFromURL()
	// list targets
	if id == 0 {
		targets, err := dao.GetAllRepTargets()
		if err != nil {
			log.Errorf("failed to get all targets: %v", err)
			t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
		t.Data["json"] = targets
		t.ServeJSON()
		return
	}

	target, err := dao.GetRepTarget(id)
	if err != nil {
		log.Errorf("failed to get target %d: %v", id, err)
		t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	if target == nil {
		t.CustomAbort(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	}

	t.Data["json"] = target
	t.ServeJSON()
}

// Post
func (t *TargetAPI) Post() {
	target := &models.RepTarget{}
	t.DecodeJSONReq(target)

	if len(target.Name) == 0 || len(target.URL) == 0 {
		t.CustomAbort(http.StatusBadRequest, "name or URL is nil")
	}

	id, err := dao.AddRepTarget(*target)
	if err != nil {
		log.Errorf("failed to add target: %v", err)
		t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	t.Redirect(http.StatusCreated, strconv.FormatInt(id, 10))
}

// Put ...
func (t *TargetAPI) Put() {
	id := t.getIDFromURL()
	if id == 0 {
		t.CustomAbort(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	}

	target := &models.RepTarget{}
	t.DecodeJSONReq(target)

	if target.ID != id {
		t.CustomAbort(http.StatusBadRequest, "IDs mismatch")
	}

	if err := dao.UpdateRepTarget(*target); err != nil {
		log.Errorf("failed to update target %d: %v", id, err)
		t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}
}

func (t *TargetAPI) getIDFromURL() int64 {
	idStr := t.Ctx.Input.Param("id")
	if len(idStr) == 0 {
		return 0
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Errorf("failed to get ID from URL: %v", err)
		t.CustomAbort(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return id
}
