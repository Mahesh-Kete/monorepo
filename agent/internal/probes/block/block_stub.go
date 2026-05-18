//go:build !linux

package block

import (
	"errors"
	gonet "net"
)

type BlockProgram struct{}

func (b *BlockProgram) Load(_ string) error {
	return errors.New("citadel block program requires linux (eBPF); rebuild on the runner")
}

func (b *BlockProgram) Block(_ gonet.IP) error   { return nil }
func (b *BlockProgram) Unblock(_ gonet.IP) error { return nil }
func (b *BlockProgram) Close() error             { return nil }
