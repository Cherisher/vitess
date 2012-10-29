// Copyright 2012, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zkwrangler

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"code.google.com/p/vitess/go/relog"
	tm "code.google.com/p/vitess/go/vt/tabletmanager"
	"code.google.com/p/vitess/go/zk"
)

func (wr *Wrangler) ExecuteHook(zkTabletPath string, hook *tm.Hook) (hookResult *tm.HookResult, err error) {
	if strings.Contains(hook.Name, "/") {
		return nil, fmt.Errorf("hook name cannot have a '/' in it")
	}
	ti, err := tm.ReadTablet(wr.zconn, zkTabletPath)
	if err != nil {
		return nil, err
	}
	return wr.ExecuteTabletInfoHook(ti, hook)
}

func (wr *Wrangler) ExecuteTabletInfoHook(ti *tm.TabletInfo, hook *tm.Hook) (hookResult *tm.HookResult, err error) {

	zkReplyPath := "hook_result.json"
	actionPath, err := wr.ai.ExecuteHook(ti.Path(), zkReplyPath, hook)
	if err != nil {
		return nil, err
	}
	err = wr.ai.WaitForCompletion(actionPath, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	zkReplyPath = tm.TabletActionToReplyPath(actionPath, zkReplyPath)
	data, _, err := wr.zconn.Get(zkReplyPath)
	if err != nil {
		return nil, err
	}
	hr := new(tm.HookResult)
	if err = json.Unmarshal([]byte(data), hr); err != nil {
		return nil, err
	}
	if err = zk.DeleteRecursive(wr.zconn, path.Dir(zkReplyPath), -1); err != nil {
		relog.Error("Cannot delete action reply %v", zkReplyPath)
	}
	return hr, nil
}

// Execute a hook and returns an error only if the hook failed, not if
// the hook doesn't exist.
func (wr *Wrangler) ExecuteOptionalTabletInfoHook(ti *tm.TabletInfo, hook *tm.Hook) (err error) {
	hr, err := wr.ExecuteTabletInfoHook(ti, hook)
	if err != nil {
		return err
	}

	if hr.ExitStatus == tm.HOOK_DOES_NOT_EXIST {
		relog.Info("Hook %v doesn't exist on tablet %v", hook.Name, ti.Path())
		return nil
	}

	if hr.ExitStatus != tm.HOOK_SUCCESS {
		return fmt.Errorf("Hook %v failed(%v): %v", hook.Name, hr.ExitStatus, hr.Stderr)
	}

	return nil
}