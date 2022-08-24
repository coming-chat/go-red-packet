package redpacket

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/coming-chat/wallet-SDK/core/base"
	"github.com/coming-chat/wallet-SDK/core/eth"
	"github.com/ethereum/go-ethereum/common"
)

const RedPacketABI = `[{"inputs":[{"internalType":"address","name":"_admin","type":"address"},{"internalType":"address","name":"_beneficiary","type":"address"},{"internalType":"uint256","name":"_base_fee","type":"uint256"}],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"_old","type":"address"},{"indexed":false,"internalType":"address","name":"_new","type":"address"}],"name":"AdminChanged","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"_old","type":"address"},{"indexed":false,"internalType":"address","name":"_new","type":"address"}],"name":"BeneficiaryChanged","type":"event"},{"inputs":[{"internalType":"uint256","name":"id","type":"uint256"},{"internalType":"address","name":"maybe_creator","type":"address"}],"name":"close","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"contract IERC20","name":"token","type":"address"},{"internalType":"uint256","name":"count","type":"uint256"},{"internalType":"uint256","name":"total_balance","type":"uint256"}],"name":"create","outputs":[],"stateMutability":"payable","type":"function"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"_fee","type":"uint256"}],"name":"NewBasePrepaidFee","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"_id","type":"uint256"},{"indexed":false,"internalType":"contract IERC20","name":"_token","type":"address"},{"indexed":false,"internalType":"uint256","name":"_count","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"_balance","type":"uint256"}],"name":"NewRedEnvelop","type":"event"},{"inputs":[{"internalType":"uint256","name":"id","type":"uint256"},{"internalType":"address[]","name":"luck_accounts","type":"address[]"},{"internalType":"uint256[]","name":"balances","type":"uint256[]"}],"name":"open","outputs":[],"stateMutability":"nonpayable","type":"function"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"previousOwner","type":"address"},{"indexed":true,"internalType":"address","name":"newOwner","type":"address"}],"name":"OwnershipTransferred","type":"event"},{"inputs":[],"name":"renounceOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"new_admin","type":"address"}],"name":"set_admin","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"new_beneficiary","type":"address"}],"name":"set_beneficiary","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"new_fee","type":"uint256"}],"name":"set_prepaid_fee","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"newOwner","type":"address"}],"name":"transferOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"_id","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"_remain_count","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"_remain_balance","type":"uint256"}],"name":"UpdateRedEnvelop","type":"event"},{"inputs":[],"name":"admin","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"base_prepaid_fee","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"beneficiary","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"count","type":"uint256"}],"name":"calc_prepaid_fee","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"id","type":"uint256"}],"name":"is_valid","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"max_count","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"next_id","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"owner","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"red_envelop_infos","outputs":[{"internalType":"contract IERC20","name":"token","type":"address"},{"internalType":"uint256","name":"remain_count","type":"uint256"},{"internalType":"uint256","name":"remain_balance","type":"uint256"}],"stateMutability":"view","type":"function"}]`

// ethRedPacketContract implement RedPacketContract interface
type ethRedPacketContract struct {
	chain   eth.IChain
	address string
}

func NewEthRedPacketContract(chain eth.IChain, contractAddress string) RedPacketContract {
	return &ethRedPacketContract{chain: chain, address: contractAddress}
}

func (contract *ethRedPacketContract) EstimateFee(rpa *RedPacketAction) (string, error) {
	switch rpa.Method {
	case RPAMethodCreate:
		count := rpa.CreateParams.Count
		rate := 200.0
		switch {
		case count <= 10:
			rate = 4
		case count <= 100:
			rate = 16
		case count <= 1000:
			rate = 200
		}
		feeFloat := big.NewFloat(0.025 * rate)
		feeFloat.Mul(feeFloat, big.NewFloat(1e18))
		feeInt, _ := feeFloat.Int(big.NewInt(0))
		return feeInt.String(), nil
	default:
		return "0", nil
	}
}

func (contract *ethRedPacketContract) EstimateGasFee(account base.Account, rpa *RedPacketAction) (string, error) {
	params, err := contract.packParams(rpa)
	if err != nil {
		return "", err
	}
	data, err := eth.EncodeContractData(RedPacketABI, rpa.Method, params...)
	if err != nil {
		return "", err
	}

	ethChain, err := contract.chain.GetEthChain()
	if err != nil {
		return "", err
	}

	price, err := ethChain.SuggestGasPrice()
	if err != nil {
		return "", err
	}

	value, err := contract.EstimateFee(rpa)
	if err != nil {
		return "", err
	}

	msg := eth.NewCallMsg()
	msg.SetFrom(account.Address())
	msg.SetTo(contract.address)
	msg.SetGasPrice(price)
	msg.SetData(data)
	msg.SetValue(value)

	gasLimit, err := contract.chain.EstimateGasLimit(msg)
	if err != nil {
		gasLimit = &base.OptionalString{Value: "200000"}
		err = nil
	}

	priceInt, ok := big.NewInt(0).SetString(price, 10)
	if !ok {
		return "", errors.New("invalid gas price")
	}
	limitInt, ok := big.NewInt(0).SetString(gasLimit.Value, 10)
	if !ok {
		return "", errors.New("invalid gas limit")
	}

	return priceInt.Mul(priceInt, limitInt).String(), nil
}

func (contract *ethRedPacketContract) FetchRedPacketCreationDetail(hash string) (*RedPacketDetail, error) {
	detail, err := contract.fetchRedPacketCreationDetail(hash)
	if err != nil {
		return nil, err
	}
	return &RedPacketDetail{
		TransactionDetail: detail.TransactionDetail,
		AmountName:        detail.AmountName,
		RedPacketAmount:   detail.RedPacketAmount,
		AmountDecimal:     detail.AmountDecimal,
	}, nil
}

func (contract *ethRedPacketContract) SendTransaction(account base.Account, rpa *RedPacketAction) (string, error) {
	params, err := contract.packParams(rpa)
	if err != nil {
		return "", err
	}
	data, err := eth.EncodeContractData(RedPacketABI, rpa.Method, params...)
	if err != nil {
		return "", err
	}

	value, err := contract.EstimateFee(rpa)
	if err != nil {
		return "", err
	}

	return contract.chain.SubmitTransactionData(account, contract.address, data, value)
}

func (contract *ethRedPacketContract) packParams(rpa *RedPacketAction) ([]interface{}, error) {
	switch rpa.Method {
	case RPAMethodCreate:
		if rpa.CreateParams == nil {
			return nil, errors.New("invalid create params")
		}
		addr := common.HexToAddress(rpa.CreateParams.TokenAddress)
		c := big.NewInt(int64(rpa.CreateParams.Count))
		a, ok := big.NewInt(0).SetString(rpa.CreateParams.Amount, 10)
		if !ok {
			return nil, fmt.Errorf("invalid red packet amount %v", rpa.CreateParams.Amount)
		}
		return []interface{}{addr, c, a}, nil
	case RPAMethodOpen:
		if rpa.OpenParams == nil {
			return nil, errors.New("invalid open params")
		}
		id := big.NewInt(rpa.OpenParams.PacketId)
		if len(rpa.OpenParams.Addresses) != len(rpa.OpenParams.Amounts) {
			return nil, fmt.Errorf("the number of opened addresses is not the same as the amount")
		}
		addrs := make([]common.Address, len(rpa.OpenParams.Addresses))
		for index, address := range rpa.OpenParams.Addresses {
			addrs[index] = common.HexToAddress(address)
		}
		amountInts := make([]*big.Int, len(rpa.OpenParams.Amounts))
		for index, amount := range rpa.OpenParams.Amounts {
			aInt, ok := big.NewInt(0).SetString(amount, 10)
			if !ok {
				return nil, fmt.Errorf("invalid red packet amount %v", amount)
			}
			amountInts[index] = aInt
		}
		return []interface{}{id, addrs, amountInts}, nil
	case RPAMethodClose:
		if rpa.CloseParams == nil {
			return nil, errors.New("invalid close params")
		}
		id := big.NewInt(rpa.CloseParams.PacketId)
		addr := common.HexToAddress(rpa.CloseParams.Creator)
		return []interface{}{id, addr}, nil
	default:
		return nil, errors.New("invalid method")
	}
}

func (contract *ethRedPacketContract) fetchRedPacketCreationDetail(hash string) (*RedPacketDetail, error) {
	chain, err := contract.chain.GetEthChain()
	if err != nil {
		return nil, err
	}

	detail, msg, err := chain.FetchTransactionDetail(hash)
	if err != nil {
		return nil, err
	}
	redDetail := &RedPacketDetail{detail, "", 0, ""}
	if data := msg.Data(); len(data) > 0 {
		method, params, err_ := eth.DecodeContractParams(RedPacketABI, data)
		if err_ != nil {
			return nil, err_
		}
		if method == RPAMethodCreate {
			feeInt, ok := big.NewInt(0).SetString(detail.EstimateFees, 10)
			if !ok {
				feeInt = big.NewInt(0)
			}
			valueInt, ok := big.NewInt(0).SetString(detail.Amount, 10)
			if !ok {
				valueInt = big.NewInt(0)
			}
			feeInt = feeInt.Add(feeInt, valueInt)

			redDetail.EstimateFees = feeInt.String()
			redDetail.Amount = params[2].(*big.Int).String()
			redDetail.RedPacketAmount = redDetail.Amount
			erc20Address := params[0].(common.Address).String()
			redDetail.AmountName, _ = chain.TokenName(erc20Address)
			redDetail.AmountDecimal, _ = chain.TokenDecimal(erc20Address)
		}
	}

	return redDetail, nil
}
