// Package i18n 은 go-i18n 기반 다국어 메시지를 제공한다. 기본 언어는 영어이며
// en/ko/ja/zh 를 지원한다. 원본 Rust 포트의 rust-i18n 계층에 대응한다.
package i18n

import (
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	bundle    *goi18n.Bundle
	localizer *goi18n.Localizer
)

// 메시지 정의: id -> 언어별 문자열. {{.name}} 템플릿을 사용한다.
var messages = map[string]map[string]string{
	"error.generic": {
		"en": "error: {{.error}}",
		"ko": "오류: {{.error}}",
		"ja": "エラー: {{.error}}",
		"zh": "错误: {{.error}}",
	},
	"error.git_open": {
		"en": "failed to open the Git repository",
		"ko": "Git 저장소를 열지 못했습니다",
		"ja": "Git リポジトリを開けませんでした",
		"zh": "无法打开 Git 仓库",
	},
	"error.calc_failed": {
		"en": "version calculation failed",
		"ko": "버전 계산에 실패했습니다",
		"ja": "バージョン計算に失敗しました",
		"zh": "版本计算失败",
	},
	"log.target_path": {
		"en": "target path: {{.path}}",
		"ko": "대상 경로: {{.path}}",
		"ja": "対象パス: {{.path}}",
		"zh": "目标路径: {{.path}}",
	},
	"log.result_written": {
		"en": "result written to {{.path}}",
		"ko": "결과를 {{.path}} 에 기록했습니다",
		"ja": "結果を {{.path}} に書き込みました",
		"zh": "结果已写入 {{.path}}",
	},
	"cli.about": {
		"en": "Calculate a semantic version from Git history (GitVersion Go port)",
		"ko": "Git 히스토리로부터 시맨틱 버전을 계산합니다 (GitVersion Go 포트)",
		"ja": "Git 履歴からセマンティックバージョンを計算します (GitVersion Go 移植版)",
		"zh": "根据 Git 历史计算语义化版本 (GitVersion Go 移植版)",
	},
}

var supported = []string{"en", "ko", "ja", "zh"}

// Init 은 lang(또는 환경변수 LANG/LC_ALL)으로 로케일을 정한다.
func Init(lang string) {
	bundle = goi18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	for _, l := range supported {
		var msgs []*goi18n.Message
		for id, byLang := range messages {
			if text, ok := byLang[l]; ok {
				msgs = append(msgs, &goi18n.Message{ID: id, Other: text})
			}
		}
		tag := language.Make(l)
		_ = bundle.AddMessages(tag, msgs...)
	}

	chosen := detectLang(lang)
	localizer = goi18n.NewLocalizer(bundle, chosen, "en")
}

func detectLang(lang string) string {
	candidates := []string{lang, os.Getenv("LC_ALL"), os.Getenv("LANG")}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// "ko_KR.UTF-8" / "ko-KR" → "ko".
		c = strings.ToLower(c)
		for _, sep := range []string{".", "_", "-"} {
			if i := strings.Index(c, sep); i >= 0 {
				c = c[:i]
			}
		}
		for _, s := range supported {
			if c == s {
				return s
			}
		}
	}
	return "en"
}

// T 는 메시지 id 를 현재 로케일로 변환한다. data 는 템플릿 인자.
func T(id string, data map[string]interface{}) string {
	if localizer == nil {
		Init("")
	}
	out, err := localizer.Localize(&goi18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: data,
	})
	if err != nil {
		return id
	}
	return out
}
