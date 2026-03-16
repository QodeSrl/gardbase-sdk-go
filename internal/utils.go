package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	gardbErrors "github.com/QodeSrl/gardbase-sdk-go/gardb/errors"
	"github.com/QodeSrl/gardbase/pkg/api/objects"
	"github.com/QodeSrl/gardbase/pkg/crypto"
)

func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

type Index struct {
	Name       objects.IndexName
	HashValue  any
	RangeValue any
}

// Note: this is only the index's token prefix, the object ID is appended to the token in the server to ensure uniqueness across objects with the same index values
func EncryptIndex(index Index, tableHash string, iek []byte) (objects.Index, error) {
	idx := objects.Index{
		Name: index.Name,
	}
	var context string
	var indexNameForErrors string
	tokenLength := 0
	if index.Name.RangeField != nil {
		context = fmt.Sprintf("%s:%s:%s", tableHash, index.Name.HashField, *index.Name.RangeField)
		indexNameForErrors = fmt.Sprintf("%s:%s", index.Name.HashField, *index.Name.RangeField)
	} else {
		context = fmt.Sprintf("%s:%s", tableHash, index.Name.HashField)
		indexNameForErrors = index.Name.HashField
	}
	// encrypt hash value with det enc using IEK
	hashValBytes, err := json.Marshal(index.HashValue)
	if err != nil {
		return idx, fmt.Errorf("%w: (index %s) failed to marshal hash value: %v", gardbErrors.ErrValidation, indexNameForErrors, err)
	}
	encryptedHashVal, err := crypto.EncryptObjectDeterministicFixed(hashValBytes, context, iek)
	if err != nil {
		return idx, fmt.Errorf("%w: (index %s) failed to encrypt hash value: %v", gardbErrors.ErrEncryption, indexNameForErrors, err)
	}
	tokenLength += len(encryptedHashVal)

	encryptedRangeVal := []byte{}
	if index.Name.RangeField != nil {
		val, err := crypto.NormalizeValue(index.RangeValue)
		if err != nil {
			return idx, fmt.Errorf("%w: (index %s) failed to normalize range value: %v", gardbErrors.ErrValidation, indexNameForErrors, err)
		}
		encryptedRangeVal, err = crypto.EncryptObjectLinearOPE(val, iek)
		if err != nil {
			return idx, fmt.Errorf("%w: (index %s) failed to encrypt range value: %v", gardbErrors.ErrEncryption, indexNameForErrors, err)
		}
		tokenLength += len(encryptedRangeVal)
	}

	token := make([]byte, 0, tokenLength)

	token = append(token, encryptedHashVal...)
	token = append(token, encryptedRangeVal...)

	idx.Token = token
	return idx, nil
}

func EncryptIndexes(indexes []Index, schemaName string, iek []byte) ([]objects.Index, error) {
	encryptedIndexes := make([]objects.Index, len(indexes))
	for k, v := range indexes {
		encryptedIdx, err := EncryptIndex(v, schemaName, iek)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt index %d: %v", k, err)
		}
		encryptedIndexes[k] = encryptedIdx
	}
	return encryptedIndexes, nil
}
