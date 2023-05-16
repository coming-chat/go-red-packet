package redpacket

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/coming-chat/wallet-SDK/core/base"
)

const (
	RPAMethodCreate = "create"
	RPAMethodOpen   = "open"
	RPAMethodClose  = "close"
)

type RedPacketContract interface {
	SendTransaction(base.Account, *RedPacketAction) (string, error)
	FetchRedPacketCreationDetail(hash string) (*RedPacketDetail, error)
	EstimateFee(*RedPacketAction) (string, error)                  // create red packet fee
	EstimateGasFee(base.Account, *RedPacketAction) (string, error) // gas fee = gasPrice * gasLimit
}

type RedPacketAction struct {
	Method string

	CreateParams *RedPacketCreateParams
	OpenParams   *RedPacketOpenParams
	CloseParams  *RedPacketCloseParams
}

func (a *RedPacketAction) TokenAddress() string {
	switch a.Method {
	case RPAMethodCreate:
		return a.CreateParams.TokenAddress
	case RPAMethodOpen:
		return a.OpenParams.TokenAddress
	case RPAMethodClose:
		return a.CloseParams.TokenAddress
	default:
		return ""
	}
}

type RedPacketCreateParams struct {
	TokenAddress string // erc20 tokenAddress, aptos coin type
	Count        int
	Amount       string
}
type RedPacketOpenParams struct {
	TokenAddress   string
	PacketId       int64  // aptos/eth use packetId
	PacketObjectId string // sui use packetObjectId
	Addresses      []string
	Amounts        []string
}
type RedPacketCloseParams struct {
	TokenAddress   string
	PacketId       int64  // aptos/eth use packetId
	PacketObjectId string // sui use packetObjectId
	Creator        string
}

type RedPacketDetail struct {
	*base.TransactionDetail

	AmountName      string
	AmountDecimal   int16
	RedPacketAmount string // 最后加入到红包里的 Amount，也即用户能够抢的那部分的 Amount
	ChainName       string
}

// 用户发红包 的操作
func NewRedPacketActionCreate(tokenAddress string, count int, amount string) (*RedPacketAction, error) {
	_, ok := big.NewInt(0).SetString(amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid red packet amount %v", amount)
	}
	if tokenAddress == "" {
		return nil, fmt.Errorf("tokenAddress must not empty")
	}
	return &RedPacketAction{
		Method: RPAMethodCreate,
		CreateParams: &RedPacketCreateParams{
			TokenAddress: tokenAddress,
			Count:        count,
			Amount:       amount,
		},
	}, nil
}

// 批量打开红包 的操作
func NewRedPacketActionOpen(tokenAddress string, packetId int64, addresses []string, amounts []string) (*RedPacketAction, error) {
	if len(addresses) != len(amounts) {
		return nil, fmt.Errorf("the number of opened addresses is not the same as the amount")
	}
	for _, amount := range amounts {
		_, ok := big.NewInt(0).SetString(amount, 10)
		if !ok {
			return nil, fmt.Errorf("invalid red packet amount %v", amount)
		}
	}
	return &RedPacketAction{
		Method: RPAMethodOpen,
		OpenParams: &RedPacketOpenParams{
			TokenAddress: tokenAddress,
			PacketId:     packetId,
			Addresses:    addresses,
			Amounts:      amounts,
		},
	}, nil
}

func NewSuiRedpacketActionOpen(tokenAddress string, packetObjectId string, addresses []string, amounts []string) (*RedPacketAction, error) {
	if len(addresses) != len(amounts) {
		return nil, fmt.Errorf("the number of opened addresses is not the same as the amount")
	}
	for _, amount := range amounts {
		_, ok := big.NewInt(0).SetString(amount, 10)
		if !ok {
			return nil, fmt.Errorf("invalid red packet amount %v", amount)
		}
	}
	return &RedPacketAction{
		Method: RPAMethodOpen,
		OpenParams: &RedPacketOpenParams{
			TokenAddress:   tokenAddress,
			Addresses:      addresses,
			Amounts:        amounts,
			PacketObjectId: packetObjectId,
		},
	}, nil
}

// NewRedPacketActionClose close red packet
// add empty arg to distinct with NewRedPacketActionCreate signature when build jar
func NewRedPacketActionClose(tokenAddress string, packetId int64, creator string, _ string) (*RedPacketAction, error) {
	return &RedPacketAction{
		Method: RPAMethodClose,
		CloseParams: &RedPacketCloseParams{
			TokenAddress: tokenAddress,
			PacketId:     packetId,
			Creator:      creator,
		},
	}, nil
}

func NewSuiRedPacketActionClose(tokenAddress string, packetObjectId string, creator string, _ string) (*RedPacketAction, error) {
	return &RedPacketAction{
		Method: RPAMethodClose,
		CloseParams: &RedPacketCloseParams{
			TokenAddress:   tokenAddress,
			PacketObjectId: packetObjectId,
			Creator:        creator,
		},
	}, nil
}

func (d *RedPacketDetail) JsonString() string {
	bytes, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func NewRedPacketDetail() *RedPacketDetail {
	return &RedPacketDetail{
		TransactionDetail: &base.TransactionDetail{},
	}
}

func NewRedPacketDetailWithJsonString(s string) (*RedPacketDetail, error) {
	bytes := []byte(s)
	var d = RedPacketDetail{}
	err := json.Unmarshal(bytes, &d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}
