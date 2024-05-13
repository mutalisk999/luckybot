package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"luckybot/app/storage"
)

// BackupRequest 备份数据库请求
type BackupRequest struct {
	Tonce int64 `json:"tonce"` // 时间戳
}

// Backup 备份数据库
func Backup(w http.ResponseWriter, r *http.Request) {
	// 跨域访问
	allowAccessControl(w)

	// 验证权限
	sessionID, data, ok := authentication(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(makeErrorRespone("", ""))
		return
	}

	// 解析请求参数
	var request BackupRequest
	if err := json.Unmarshal(data, &request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(makeErrorRespone(sessionID, err.Error()))
		return
	}

	// 备份数据库
	buf := bytes.NewBuffer(make([]byte, 0, bytes.MinRead))
	size, err := storage.Backup(buf)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(makeErrorRespone(sessionID, err.Error()))
		return
	}

	// 返回数据库
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="master.db"`)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}
