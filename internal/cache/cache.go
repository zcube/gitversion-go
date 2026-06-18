// Package cache 는 버전 계산 결과를 디스크에 캐시한다.
//
// 원본 GitVersion.Core/VersionCalculation/Caching 대응. 저장소 상태(refs + HEAD),
// 설정 파일 내용, overrideconfig 값을 SHA1 으로 해시한 키로 결과를
// <.git>/gitversion_cache/<키>.json 에 저장하고 재사용한다. 저장소 상태나 설정이
// 바뀌면 키가 달라져 자동으로 무효화된다.
package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/zcube/go-gitversion/internal/git"
	"github.com/zcube/go-gitversion/internal/output"
)

// ComputeKey 는 (refs 스냅샷 + HEAD + 설정파일 내용 + overrideconfig)의 SHA1 hex.
// 원본 GitVersionCacheKeyFactory 의 4개 구성요소에 대응한다. GitVersion 자체 버전은
// 키에 포함하지 않는다(개발 중에는 --nocache 사용).
func ComputeKey(repo *git.GitRepo, configPath string, overrides []string) string {
	var content []byte
	if configPath != "" {
		content, _ = os.ReadFile(configPath)
	}
	return ComputeKeyContent(repo, content, overrides)
}

// ComputeKeyContent 는 설정 파일 경로 대신 설정 내용(바이트)을 직접 받아 키를 만든다.
// 인라인/객체 설정으로 계산할 때 사용한다.
func ComputeKeyContent(repo *git.GitRepo, configContent []byte, overrides []string) string {
	h := sha1.New()

	for _, line := range repo.RefsSnapshot() {
		h.Write([]byte(line))
		h.Write([]byte("\n"))
	}
	h.Write([]byte("--head--"))
	h.Write([]byte(repo.HeadRefName()))
	if head, err := repo.HeadCommit(); err == nil {
		h.Write([]byte(head.Sha))
	}
	h.Write([]byte("--config--"))
	h.Write(configContent)
	h.Write([]byte("--override--"))
	for _, o := range overrides {
		h.Write([]byte(o))
		h.Write([]byte("\n"))
	}

	return hex.EncodeToString(h.Sum(nil))
}

func cacheFile(repo *git.GitRepo, key string) string {
	return filepath.Join(repo.GitDir(), "gitversion_cache", key+".json")
}

// Load 는 캐시에서 변수를 로드한다. 없거나 손상되면 nil(손상 시 삭제).
func Load(repo *git.GitRepo, key string) *output.VersionVariables {
	path := cacheFile(repo, key)
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Debug("cache miss: " + path)
		return nil
	}
	vars, err := output.FromJSON(data)
	if err != nil {
		slog.Warn("cache corrupt(삭제): " + path)
		_ = os.Remove(path)
		return nil
	}
	slog.Debug("cache hit: " + path)
	return vars
}

// Store 는 변수를 캐시에 기록한다.
func Store(repo *git.GitRepo, key string, vars *output.VersionVariables) {
	path := cacheFile(repo, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	json, err := vars.ToJSON()
	if err != nil {
		slog.Warn("cache 직렬화 실패")
		return
	}
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		slog.Warn("cache 기록 실패: " + path)
		return
	}
	slog.Debug("cache write: " + path)
}

// Clear 는 디스크 캐시 디렉터리를 삭제하고 삭제한 파일 수를 반환한다.
func Clear(repo *git.GitRepo) (int, error) {
	dir := filepath.Join(repo.GitDir(), "gitversion_cache")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := len(entries)
	if err := os.RemoveAll(dir); err != nil {
		return 0, err
	}
	return count, nil
}
