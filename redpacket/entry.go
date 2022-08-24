package redpacket

import (
	"errors"

	"github.com/coming-chat/wallet-SDK/core/aptos"
	"github.com/coming-chat/wallet-SDK/core/base"
	"github.com/coming-chat/wallet-SDK/core/eth"
)

const (
	ChainTypeEth   = "eth"
	ChainTypeAptos = "aptos"
)

func NewRedPacketContract(chainType string, chain base.Chain, contractAddress string) (RedPacketContract, error) {
	switch chainType {
	case ChainTypeEth:
		if ethChain, ok := chain.(eth.IChain); ok {
			return NewEthRedPacketContract(ethChain, contractAddress), nil
		} else {
			return nil, errors.New("invalid chain object")
		}
	case ChainTypeAptos:
		if aptosChain, ok := chain.(aptos.IChain); ok {
			return NewAptosRedPacketContract(aptosChain, contractAddress), nil
		} else {
			return nil, errors.New("invalid chain object")
		}
	default:
		return nil, errors.New("unsupport chain type")
	}
}
