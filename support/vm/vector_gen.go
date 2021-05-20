package vm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
)

// Update this when generating new vectors for a new filecoin network version
const defaultNetworkName = "hyperdrive"

// option functions for setting vector generator field cleanly
// write state tree to encoded car file
// deal with top of state tree not matching filecoin protocol
// persisting runtime values

type TestVector struct {
	ID string

	StartState     []byte
	StartStateTree cid.Cid
	Message        *ChainMessage

	Receipt      MessageResult
	EndStateTree cid.Cid

	// Runtime values
	Epoch      abi.ChainEpoch
	Version    network.Version
	CircSupply abi.TokenAmount
}

func (tv *TestVector) MarshalJSON() ([]byte, error) {
	tvs, err := newTestVectorSerial(tv)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&tvs)
}

type Option func(tv *TestVector) error

func SetID(id string) Option {
	return func(tv *TestVector) error {
		tv.ID = id
		return nil
	}
}

// func SetState(bs blockstore.BlockStore, stateRoot cid.Cid) Option {
// 	return func(tv *TestVector) error {
// 		// todo write car
// 		// tv.StartState = carBytes
// 		return nil
// 	}
// }

func SetEpoch(e abi.ChainEpoch) Option {
	return func(tv *TestVector) error {
		tv.Epoch = e
		return nil
	}
}

func SetNetworkVersion(nv network.Version) Option {
	return func(tv *TestVector) error {
		tv.Version = nv
		return nil
	}
}

func SetCircSupply(circSupply big.Int) Option {
	return func(tv *TestVector) error {
		tv.CircSupply = circSupply
		return nil
	}
}

func SetStartStateTree(root cid.Cid) Option {
	return func(tv *TestVector) error {
		// TODO wrap with the toplevel state tree object to get tru root cid
		tv.StartStateTree = root
		return nil
	}
}

func SetEndStateTree(root cid.Cid) Option {
	return func(tv *TestVector) error {
		// TODO wrap with the toplevel state tree object to get tru root cid
		tv.EndStateTree = root
		return nil
	}
}

func SetMessage(from, to address.Address, nonce uint64, value big.Int, method abi.MethodNum, params interface{}) Option {
	return func(tv *TestVector) error {
		msg, err := makeChainMessage(from, to, nonce, value, method, params)
		if err != nil {
			return err
		}
		tv.Message = msg
		return nil
	}
}

func SetReceipt(res MessageResult) Option {
	return func(tv *TestVector) error {
		tv.Receipt = res
		return nil
	}
}

func StartConditions(v *VM, id string) []Option {
	var opts []Option
	opts = append(opts, SetEpoch(v.GetEpoch()))
	opts = append(opts, SetCircSupply(v.GetCirculatingSupply()))
	opts = append(opts, SetNetworkVersion(v.networkVersion))
	opts = append(opts, SetStartStateTree(v.StateRoot()))
	opts = append(opts, SetID(id))

	// TODO SetState

	return opts
}

func EndConditions(v VM, res MessageResult) []Option {
	var opts []Option

	return opts
}

//
// Internal types for serialization
// Taken from https://github.com/filecoin-project/test-vectors/blob/master/schema/schema.go
//

type generationData struct {
	Source string `json:"source"`
}

type metadata struct {
	ID  string           `json:"id"`
	Gen []generationData `json:"gen"`
}

type variant struct {
	// ID of the variant, usually the codename of the upgrade.
	ID             string `json:"id"`
	Epoch          int64  `json:"epoch"`
	NetworkVersion uint   `json:"nv"`
}

type preconditions struct {
	Variants   []variant        `json:"variants"`
	StateTree  *stateTreeSerial `json:"state_tree,omitempty"`
	BaseFee    *big.Int         `json:"basefee,omitempty"`
	CircSupply *big.Int         `json:"circ_supply,omitempty"`
}

type base64EncodedBytes []byte

func (b base64EncodedBytes) String() string {
	return base64.StdEncoding.EncodeToString(b)
}

// MarshalJSON implements json.Marshal for Base64EncodedBytes
func (b base64EncodedBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}

type messageSerial struct {
	Bytes base64EncodedBytes `json:"bytes"`
}
type stateTreeSerial struct {
	RootCID cid.Cid `json:"root_cid"`
}

// Receipt represents a receipt to match against.
type receiptSerial struct {
	// ExitCode must be interpreted by the driver as an exitcode.ExitCode
	// in Lotus, or equivalent type in other implementations.
	ExitCode    int64              `json:"exit_code"`
	ReturnValue base64EncodedBytes `json:"return"`
	GasUsed     int64              `json:"gas_used"`
}

// Postconditions contain a representation of VM state at th end of the test
type postconditions struct {
	StateTree *stateTreeSerial `json:"state_tree"`
	Receipts  []*receiptSerial `json:"receipts"`
}

type testVectorSerial struct {
	Class string `json:"class"`

	Meta *metadata `json:"_meta"`

	// CAR binary data to be loaded into the test environment. Should
	// contain objects of entire state tree
	CAR base64EncodedBytes `json:"car"`

	Pre *preconditions `json:"preconditions"`

	ApplyMessages []messageSerial `json:"apply_messages,omitempty"`

	Post *postconditions `json:"postconditions"`
}

func newTestVectorSerial(tv *TestVector) (*testVectorSerial, error) {
	zero := big.Zero()
	circSupply := tv.CircSupply
	var buf bytes.Buffer
	if err := tv.Message.MarshalCBOR(&buf); err != nil {
		return nil, err
	}
	msgBytes := buf.Bytes()
	if err := tv.Receipt.Ret.MarshalCBOR(&buf); err != nil {
		return nil, err
	}
	retBytes := buf.Bytes()

	return &testVectorSerial{
		Class: "message",
		Meta: &metadata{
			ID: tv.ID,
			Gen: []generationData{
				{Source: "specs-actors_test_auto_gen"},
			},
		},
		CAR: tv.StartState,
		Pre: &preconditions{
			Variants: []variant{
				{ID: defaultNetworkName, Epoch: int64(tv.Epoch), NetworkVersion: uint(tv.Version)},
			},
			StateTree:  &stateTreeSerial{RootCID: tv.StartStateTree},
			BaseFee:    &zero,
			CircSupply: &circSupply,
		},
		ApplyMessages: []messageSerial{
			{Bytes: msgBytes},
		},
		Post: &postconditions{
			StateTree: &stateTreeSerial{RootCID: tv.EndStateTree},
			Receipts: []*receiptSerial{
				{
					ExitCode:    int64(tv.Receipt.Code),
					ReturnValue: retBytes,
					GasUsed:     tv.Receipt.GasCharged,
				},
			},
		},
	}, nil
}
