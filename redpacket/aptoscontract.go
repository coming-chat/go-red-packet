package redpacket

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/coming-chat/go-aptos/aptostypes"
	"github.com/coming-chat/wallet-SDK/core/aptos"
	"github.com/coming-chat/wallet-SDK/core/base"
)

const (
	AptosName    = "Aptos"
	AptosSymbol  = "Aptos"
	AptosDecimal = 0

	MaxGasAmount = 1000
	GasPrice     = 1
)

// aptosRedPacketContract implement RedPacketContract interface
type aptosRedPacketContract struct {
	chain   aptos.IChain
	address string
}

func NewAptosRedPacketContract(chain aptos.IChain, contractAddress string) RedPacketContract {
	return &aptosRedPacketContract{chain: chain, address: contractAddress}
}

func (contract *aptosRedPacketContract) EstimateFee(rpa *RedPacketAction) (string, error) {
	switch rpa.Method {
	case RPAMethodCreate:
		if nil == rpa.CreateParams {
			return "", errors.New("invalid create params")
		}
		amount, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return "", err
		}
		feePoint, err := contract.getFeePoint()
		if err != nil {
			return "", err
		}
		total := calcTotal(amount, uint64(feePoint))
		return strconv.FormatUint(total-amount, 10), nil
	default:
		return "", errors.New("method invalid")
	}
}

// getFeePoint get fee_point from contract by resouce
// when api support call move public function, should not use resouce
func (contract *aptosRedPacketContract) getFeePoint() (uint64, error) {
	client, err := contract.chain.GetClient()
	if err != nil {
		return 0, err
	}
	resource, err := client.GetAccountResource(contract.address, contract.address+"::red_packet::RedPackets", 0)
	if err != nil {
		return 0, err
	}
	feePoint, _ := resource.Data["fee_point"].(float64)
	return uint64(feePoint), nil
}

func (contract *aptosRedPacketContract) FetchRedPacketCreationDetail(hash string) (*RedPacketDetail, error) {
	client, err := contract.chain.GetClient()
	if err != nil {
		return nil, err
	}
	transaction, err := client.GetTransactionByHash(hash)
	if err != nil {
		return nil, err
	}
	baseTransaction, err := toBaseTransaction(transaction)
	if err != nil {
		return nil, err
	}

	if len(transaction.Payload.Arguments) < 2 {
		return nil, fmt.Errorf("invalid payload arguments, len %d", len(transaction.Payload.Arguments))
	}
	baseTransaction.Amount = transaction.Payload.Arguments[1].(string)
	redPacketDetail := &RedPacketDetail{
		TransactionDetail: baseTransaction,
		AmountName:        AptosName,
		AmountDecimal:     AptosDecimal,
	}

	return redPacketDetail, nil
}

func (contract *aptosRedPacketContract) SendTransaction(account base.Account, rpa *RedPacketAction) (string, error) {
	payload, err := contract.createPayload(rpa)
	if err != nil {
		return "", err
	}
	return contract.chain.SubmitTransactionPayload(account, payload, newProcessMaxGasTxOption(rpa))
}

func (contract *aptosRedPacketContract) createPayload(rpa *RedPacketAction) (*aptostypes.Payload, error) {
	switch rpa.Method {
	case RPAMethodCreate:
		if nil == rpa.CreateParams {
			return nil, fmt.Errorf("create params is nil")
		}
		amount, err := strconv.ParseUint(rpa.CreateParams.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("amount params is not uint64")
		}
		feePoint, err := contract.getFeePoint()
		if err != nil {
			return nil, err
		}
		amountTotal := calcTotal(amount, feePoint)
		return &aptostypes.Payload{
			Type:          aptostypes.EntryFunctionPayload,
			Function:      contract.address + "::red_packet::create",
			TypeArguments: []string{},
			Arguments: []interface{}{
				strconv.FormatInt(int64(rpa.CreateParams.Count), 10),
				strconv.FormatUint(amountTotal, 10),
			},
		}, nil
	case RPAMethodOpen:
		if nil == rpa.OpenParams {
			return nil, fmt.Errorf("open params is nil")
		}
		return &aptostypes.Payload{
			Type:          aptostypes.EntryFunctionPayload,
			Function:      contract.address + "::red_packet::open",
			TypeArguments: []string{},
			Arguments: []interface{}{
				strconv.FormatInt(int64(rpa.OpenParams.PacketId), 10),
				rpa.OpenParams.Addresses,
				rpa.OpenParams.Amounts,
			},
		}, nil
	case RPAMethodClose:
		if nil == rpa.CloseParams {
			return nil, fmt.Errorf("close params is nil")
		}
		return &aptostypes.Payload{
			Type:          aptostypes.EntryFunctionPayload,
			Function:      contract.address + "::red_packet::close",
			TypeArguments: []string{},
			Arguments: []interface{}{
				strconv.FormatInt(int64(rpa.CloseParams.PacketId), 10),
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsopported red packet method %s", rpa.Method)
	}
}

// calcTotal caculate totalAmount should send, when user want create a red packet with amount
func calcTotal(amount uint64, feePoint uint64) uint64 {
	if feePoint == 0 {
		feePoint = 250
	}
	if amount < 10000 {
		return amount
	}
	fee := amount / 10000 * feePoint
	left := uint64(0)
	right := amount / feePoint
	for left <= right {
		center := (left + right) / 2
		tmpFee := center*feePoint + fee
		tmpTotal := tmpFee + amount
		tmpC := tmpTotal - tmpTotal/10000*feePoint
		if tmpC > amount {
			right = center - 1
		} else if tmpC < amount {
			left = center + 1
		} else {
			return tmpTotal
		}
	}
	return amount
}

func toBaseTransaction(transaction *aptostypes.Transaction) (*base.TransactionDetail, error) {
	if transaction.Type != aptostypes.TypeUserTransaction ||
		transaction.Payload.Type != aptostypes.EntryFunctionPayload {
		return nil, errors.New("invalid transfer transaction.")
	}

	detail := &base.TransactionDetail{
		HashString:  transaction.Hash,
		FromAddress: transaction.Sender,
	}

	gasFee := transaction.GasUnitPrice * transaction.GasUsed
	detail.EstimateFees = strconv.FormatUint(gasFee, 10)

	args := transaction.Payload.Arguments
	if len(args) >= 2 {
		detail.ToAddress = args[0].(string)
		detail.Amount = args[1].(string)
	}

	if transaction.Success {
		detail.Status = base.TransactionStatusSuccess
	} else {
		detail.Status = base.TransactionStatusFailure
		detail.FailureMessage = transaction.VmStatus
	}

	timestamp := transaction.Timestamp / 1e6
	detail.FinishTimestamp = int64(timestamp)

	return detail, nil
}

type transactionOption struct {
	rpa *RedPacketAction
}

func newProcessMaxGasTxOption(rpa *RedPacketAction) aptos.TransactionOption {
	return &transactionOption{rpa: rpa}
}

func (o *transactionOption) Process(tx *aptostypes.Transaction) (*aptostypes.Transaction, error) {
	tx.MaxGasAmount = o.estimateGas()
	return tx, nil
}

func (o *transactionOption) estimateGas() uint64 {
	if o.rpa.Method != RPAMethodOpen {
		return MaxGasAmount
	}
	// devnet 测试结果
	// OpenPacketCount  |  GasUsed
	// 10 | 19
	// 100 | 249
	// 200 | 483
	// 500 | 1158
	// 1000 | 2355
	if len(o.rpa.OpenParams.Addresses) > 200 {
		return 2000
	} else if len(o.rpa.OpenParams.Addresses) > 500 {
		return 3000
	} else {
		return MaxGasAmount
	}
}
