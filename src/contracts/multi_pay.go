// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// MultiPayPaySummary is an auto generated low-level Go binding around an user-defined struct.
type MultiPayPaySummary struct {
	Payer       common.Address
	Executor    common.Address
	Token       common.Address
	Timestamp   uint32
	Id          uint32
	TotalAmount *big.Int
}

// MultiPayMetaData contains all meta data concerning the MultiPay contract.
var MultiPayMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint32\",\"name\":\"id\",\"type\":\"uint32\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"},{\"indexed\":false,\"internalType\":\"address[]\",\"name\":\"payees\",\"type\":\"address[]\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"message\",\"type\":\"string\"}],\"name\":\"Payment\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"_id\",\"type\":\"uint32\"},{\"internalType\":\"address\",\"name\":\"_tokenAddr\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_payer\",\"type\":\"address\"},{\"internalType\":\"uint256[]\",\"name\":\"_amounts\",\"type\":\"uint256[]\"},{\"internalType\":\"address[]\",\"name\":\"_payees\",\"type\":\"address[]\"},{\"internalType\":\"string\",\"name\":\"_message\",\"type\":\"string\"}],\"name\":\"_executePayment\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"payer\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"executor\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"token\",\"type\":\"address\"},{\"internalType\":\"uint32\",\"name\":\"timestamp\",\"type\":\"uint32\"},{\"internalType\":\"uint32\",\"name\":\"id\",\"type\":\"uint32\"},{\"internalType\":\"uint256\",\"name\":\"totalAmount\",\"type\":\"uint256\"}],\"internalType\":\"structMultiPay.PaySummary\",\"name\":\"_payload\",\"type\":\"tuple\"},{\"internalType\":\"bytes\",\"name\":\"_signature\",\"type\":\"bytes\"},{\"internalType\":\"uint256[]\",\"name\":\"_amounts\",\"type\":\"uint256[]\"},{\"internalType\":\"address[]\",\"name\":\"_payees\",\"type\":\"address[]\"},{\"internalType\":\"string\",\"name\":\"_message\",\"type\":\"string\"}],\"name\":\"delegatedPay\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"executedPaymentDigests\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint32\",\"name\":\"_id\",\"type\":\"uint32\"},{\"internalType\":\"address\",\"name\":\"_tokenAddr\",\"type\":\"address\"},{\"internalType\":\"uint256[]\",\"name\":\"_amounts\",\"type\":\"uint256[]\"},{\"internalType\":\"address[]\",\"name\":\"_payees\",\"type\":\"address[]\"},{\"internalType\":\"string\",\"name\":\"_message\",\"type\":\"string\"}],\"name\":\"pay\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// MultiPayABI is the input ABI used to generate the binding from.
// Deprecated: Use MultiPayMetaData.ABI instead.
var MultiPayABI = MultiPayMetaData.ABI

// MultiPay is an auto generated Go binding around an Ethereum contract.
type MultiPay struct {
	MultiPayCaller     // Read-only binding to the contract
	MultiPayTransactor // Write-only binding to the contract
	MultiPayFilterer   // Log filterer for contract events
}

// MultiPayCaller is an auto generated read-only Go binding around an Ethereum contract.
type MultiPayCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MultiPayTransactor is an auto generated write-only Go binding around an Ethereum contract.
type MultiPayTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MultiPayFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type MultiPayFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MultiPaySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type MultiPaySession struct {
	Contract     *MultiPay         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// MultiPayCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type MultiPayCallerSession struct {
	Contract *MultiPayCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// MultiPayTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type MultiPayTransactorSession struct {
	Contract     *MultiPayTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// MultiPayRaw is an auto generated low-level Go binding around an Ethereum contract.
type MultiPayRaw struct {
	Contract *MultiPay // Generic contract binding to access the raw methods on
}

// MultiPayCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type MultiPayCallerRaw struct {
	Contract *MultiPayCaller // Generic read-only contract binding to access the raw methods on
}

// MultiPayTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type MultiPayTransactorRaw struct {
	Contract *MultiPayTransactor // Generic write-only contract binding to access the raw methods on
}

// NewMultiPay creates a new instance of MultiPay, bound to a specific deployed contract.
func NewMultiPay(address common.Address, backend bind.ContractBackend) (*MultiPay, error) {
	contract, err := bindMultiPay(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &MultiPay{MultiPayCaller: MultiPayCaller{contract: contract}, MultiPayTransactor: MultiPayTransactor{contract: contract}, MultiPayFilterer: MultiPayFilterer{contract: contract}}, nil
}

// NewMultiPayCaller creates a new read-only instance of MultiPay, bound to a specific deployed contract.
func NewMultiPayCaller(address common.Address, caller bind.ContractCaller) (*MultiPayCaller, error) {
	contract, err := bindMultiPay(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &MultiPayCaller{contract: contract}, nil
}

// NewMultiPayTransactor creates a new write-only instance of MultiPay, bound to a specific deployed contract.
func NewMultiPayTransactor(address common.Address, transactor bind.ContractTransactor) (*MultiPayTransactor, error) {
	contract, err := bindMultiPay(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &MultiPayTransactor{contract: contract}, nil
}

// NewMultiPayFilterer creates a new log filterer instance of MultiPay, bound to a specific deployed contract.
func NewMultiPayFilterer(address common.Address, filterer bind.ContractFilterer) (*MultiPayFilterer, error) {
	contract, err := bindMultiPay(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &MultiPayFilterer{contract: contract}, nil
}

// bindMultiPay binds a generic wrapper to an already deployed contract.
func bindMultiPay(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := MultiPayMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MultiPay *MultiPayRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _MultiPay.Contract.MultiPayCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MultiPay *MultiPayRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MultiPay.Contract.MultiPayTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MultiPay *MultiPayRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MultiPay.Contract.MultiPayTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MultiPay *MultiPayCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _MultiPay.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MultiPay *MultiPayTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MultiPay.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MultiPay *MultiPayTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MultiPay.Contract.contract.Transact(opts, method, params...)
}

// ExecutedPaymentDigests is a free data retrieval call binding the contract method 0xf727675d.
//
// Solidity: function executedPaymentDigests(bytes32 ) view returns(bool)
func (_MultiPay *MultiPayCaller) ExecutedPaymentDigests(opts *bind.CallOpts, arg0 [32]byte) (bool, error) {
	var out []interface{}
	err := _MultiPay.contract.Call(opts, &out, "executedPaymentDigests", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ExecutedPaymentDigests is a free data retrieval call binding the contract method 0xf727675d.
//
// Solidity: function executedPaymentDigests(bytes32 ) view returns(bool)
func (_MultiPay *MultiPaySession) ExecutedPaymentDigests(arg0 [32]byte) (bool, error) {
	return _MultiPay.Contract.ExecutedPaymentDigests(&_MultiPay.CallOpts, arg0)
}

// ExecutedPaymentDigests is a free data retrieval call binding the contract method 0xf727675d.
//
// Solidity: function executedPaymentDigests(bytes32 ) view returns(bool)
func (_MultiPay *MultiPayCallerSession) ExecutedPaymentDigests(arg0 [32]byte) (bool, error) {
	return _MultiPay.Contract.ExecutedPaymentDigests(&_MultiPay.CallOpts, arg0)
}

// ExecutePayment is a paid mutator transaction binding the contract method 0x28ab2628.
//
// Solidity: function _executePayment(uint32 _id, address _tokenAddr, address _payer, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPayTransactor) ExecutePayment(opts *bind.TransactOpts, _id uint32, _tokenAddr common.Address, _payer common.Address, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.contract.Transact(opts, "_executePayment", _id, _tokenAddr, _payer, _amounts, _payees, _message)
}

// ExecutePayment is a paid mutator transaction binding the contract method 0x28ab2628.
//
// Solidity: function _executePayment(uint32 _id, address _tokenAddr, address _payer, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPaySession) ExecutePayment(_id uint32, _tokenAddr common.Address, _payer common.Address, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.Contract.ExecutePayment(&_MultiPay.TransactOpts, _id, _tokenAddr, _payer, _amounts, _payees, _message)
}

// ExecutePayment is a paid mutator transaction binding the contract method 0x28ab2628.
//
// Solidity: function _executePayment(uint32 _id, address _tokenAddr, address _payer, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPayTransactorSession) ExecutePayment(_id uint32, _tokenAddr common.Address, _payer common.Address, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.Contract.ExecutePayment(&_MultiPay.TransactOpts, _id, _tokenAddr, _payer, _amounts, _payees, _message)
}

// DelegatedPay is a paid mutator transaction binding the contract method 0x80fc8238.
//
// Solidity: function delegatedPay((address,address,address,uint32,uint32,uint256) _payload, bytes _signature, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPayTransactor) DelegatedPay(opts *bind.TransactOpts, _payload MultiPayPaySummary, _signature []byte, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.contract.Transact(opts, "delegatedPay", _payload, _signature, _amounts, _payees, _message)
}

// DelegatedPay is a paid mutator transaction binding the contract method 0x80fc8238.
//
// Solidity: function delegatedPay((address,address,address,uint32,uint32,uint256) _payload, bytes _signature, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPaySession) DelegatedPay(_payload MultiPayPaySummary, _signature []byte, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.Contract.DelegatedPay(&_MultiPay.TransactOpts, _payload, _signature, _amounts, _payees, _message)
}

// DelegatedPay is a paid mutator transaction binding the contract method 0x80fc8238.
//
// Solidity: function delegatedPay((address,address,address,uint32,uint32,uint256) _payload, bytes _signature, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPayTransactorSession) DelegatedPay(_payload MultiPayPaySummary, _signature []byte, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.Contract.DelegatedPay(&_MultiPay.TransactOpts, _payload, _signature, _amounts, _payees, _message)
}

// Pay is a paid mutator transaction binding the contract method 0x39dc6c57.
//
// Solidity: function pay(uint32 _id, address _tokenAddr, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPayTransactor) Pay(opts *bind.TransactOpts, _id uint32, _tokenAddr common.Address, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.contract.Transact(opts, "pay", _id, _tokenAddr, _amounts, _payees, _message)
}

// Pay is a paid mutator transaction binding the contract method 0x39dc6c57.
//
// Solidity: function pay(uint32 _id, address _tokenAddr, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPaySession) Pay(_id uint32, _tokenAddr common.Address, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.Contract.Pay(&_MultiPay.TransactOpts, _id, _tokenAddr, _amounts, _payees, _message)
}

// Pay is a paid mutator transaction binding the contract method 0x39dc6c57.
//
// Solidity: function pay(uint32 _id, address _tokenAddr, uint256[] _amounts, address[] _payees, string _message) returns()
func (_MultiPay *MultiPayTransactorSession) Pay(_id uint32, _tokenAddr common.Address, _amounts []*big.Int, _payees []common.Address, _message string) (*types.Transaction, error) {
	return _MultiPay.Contract.Pay(&_MultiPay.TransactOpts, _id, _tokenAddr, _amounts, _payees, _message)
}

// MultiPayPaymentIterator is returned from FilterPayment and is used to iterate over the raw logs and unpacked data for Payment events raised by the MultiPay contract.
type MultiPayPaymentIterator struct {
	Event *MultiPayPayment // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *MultiPayPaymentIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MultiPayPayment)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(MultiPayPayment)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *MultiPayPaymentIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MultiPayPaymentIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MultiPayPayment represents a Payment event raised by the MultiPay contract.
type MultiPayPayment struct {
	From    common.Address
	Id      uint32
	Token   common.Address
	Amounts []*big.Int
	Payees  []common.Address
	Message string
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterPayment is a free log retrieval operation binding the contract event 0xb5db0baa63b5733000aa061ff33545663099374a29f64e7c168b389d41e6e348.
//
// Solidity: event Payment(address indexed from, uint32 indexed id, address indexed token, uint256[] amounts, address[] payees, string message)
func (_MultiPay *MultiPayFilterer) FilterPayment(opts *bind.FilterOpts, from []common.Address, id []uint32, token []common.Address) (*MultiPayPaymentIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}

	logs, sub, err := _MultiPay.contract.FilterLogs(opts, "Payment", fromRule, idRule, tokenRule)
	if err != nil {
		return nil, err
	}
	return &MultiPayPaymentIterator{contract: _MultiPay.contract, event: "Payment", logs: logs, sub: sub}, nil
}

// WatchPayment is a free log subscription operation binding the contract event 0xb5db0baa63b5733000aa061ff33545663099374a29f64e7c168b389d41e6e348.
//
// Solidity: event Payment(address indexed from, uint32 indexed id, address indexed token, uint256[] amounts, address[] payees, string message)
func (_MultiPay *MultiPayFilterer) WatchPayment(opts *bind.WatchOpts, sink chan<- *MultiPayPayment, from []common.Address, id []uint32, token []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}
	var tokenRule []interface{}
	for _, tokenItem := range token {
		tokenRule = append(tokenRule, tokenItem)
	}

	logs, sub, err := _MultiPay.contract.WatchLogs(opts, "Payment", fromRule, idRule, tokenRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MultiPayPayment)
				if err := _MultiPay.contract.UnpackLog(event, "Payment", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePayment is a log parse operation binding the contract event 0xb5db0baa63b5733000aa061ff33545663099374a29f64e7c168b389d41e6e348.
//
// Solidity: event Payment(address indexed from, uint32 indexed id, address indexed token, uint256[] amounts, address[] payees, string message)
func (_MultiPay *MultiPayFilterer) ParsePayment(log types.Log) (*MultiPayPayment, error) {
	event := new(MultiPayPayment)
	if err := _MultiPay.contract.UnpackLog(event, "Payment", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
