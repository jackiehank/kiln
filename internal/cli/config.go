package cli

import (
	"fmt"

	"github.com/jackiehank/kiln/internal/config"
	"github.com/jackiehank/kiln/internal/i18n"
)

// config implements `kiln config path`: it prints where the repository config
// lives, seeding it when missing so there is always something to edit.
func (e *env) config(args []string) int {
	pos, f, _ := parse(args)
	warnUnknownFlags(e.errw, f, allowedFlags())
	if code := rejectExtra(e.errw, pos, 1, "kiln config path"); code != 0 {
		return code
	}
	sub := ""
	if len(pos) > 0 {
		sub = pos[0]
	}
	if sub != "path" {
		fmt.Fprintln(e.errw, i18n.T("用法: kiln config path"))
		return 2
	}
	path, created, err := config.Ensure(e.dir)
	if err != nil {
		fmt.Fprint(e.errw, i18n.T("定位配置失败: %v\n", err))
		return 1
	}
	fmt.Fprintln(e.out, path)
	if created {
		fmt.Fprintln(e.out, i18n.T("（原本不存在，已写入一份带注释的默认配置）"))
	}
	return 0
}
