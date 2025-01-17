// Package repository implements persistent data access and processing.
package repository

import (
	"artion-api-graphql/internal/types"
	"artion-api-graphql/internal/types/sorting"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"strings"
	"time"
)

// Token reads NFT detail from the persistent database.
func (p *Proxy) Token(contract *common.Address, tokenId *hexutil.Big) (*types.Token, error) {
	var key strings.Builder
	key.WriteString("Token")
	key.WriteString(contract.String())
	key.WriteString(tokenId.String())

	token, err, _ := p.callGroup.Do(key.String(), func() (interface{}, error) {
		defer func() {
			if r := recover(); r != nil {
				log.Criticalf("recovered from panic in token loading")
			}
		}()

		// load the token locally
		t, e := p.db.GetToken(contract, (*big.Int)(tokenId))
		if e != nil {
			return nil, e
		}

		if t == nil {
			log.Warningf("token %s / %s not found", contract.String(), tokenId.String())
			return nil, nil
		}
		return t, nil
	})
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}

	return token.(*types.Token), err
}

// MustTokenName returns name of the given token, or it's ID if the name is not available.
func (p *Proxy) MustTokenName(contract *common.Address, tokenID *hexutil.Big) string {
	t, err := p.Token(contract, tokenID)
	if err != nil {
		return tokenID.String()
	}
	if t.Name == "" {
		return tokenID.String()
	}
	return t.Name
}

// ExtendLegacyToken tries to load token metadata details from the shared legacy database.
func (p *Proxy) ExtendLegacyToken(token *types.Token) (*types.Token, error) {
	return p.shared.ExtendLegacyToken(token)
}

// TokenKnown checks if the given token exists i the database.
func (p *Proxy) TokenKnown(contract *common.Address, tokenId *big.Int) bool {
	return p.db.TokenKnown(contract, tokenId)
}

// MustTokenOwners gets the owner of the given token, if available.
func (p *Proxy) MustTokenOwners(contract *common.Address, tokenId hexutil.Big) []common.Address {
	ow, err := p.db.GetTokenOwners(*contract, tokenId)
	if err != nil {
		log.Errorf("unknown owners of %s/#%s; %s", contract.String(), tokenId.String(), err.Error())
		return []common.Address{}
	}
	return ow
}

// StoreToken puts the given token into the persistent storage.
// The function is used for both insert and update operation.
func (p *Proxy) StoreToken(token *types.Token) error {
	return p.db.StoreToken(token)
}

// UpdateTokenMetadata updates basic metadata of the NFT token.
func (p *Proxy) UpdateTokenMetadata(nft *types.Token) error {
	return p.db.UpdateTokenMetadata(nft)
}

// UpdateTokenMetadataRefreshSchedule sets the NFT metadata update schedule time.
func (p *Proxy) UpdateTokenMetadataRefreshSchedule(nft *types.Token) error {
	return p.db.UpdateTokenMetadataRefreshSchedule(nft)
}

// TokenMetadataRefreshSet pulls s set of NFT tokens scheduled to be updated up to this time.
func (p *Proxy) TokenMetadataRefreshSet(setSize int64) ([]*types.Token, error) {
	return p.db.TokenMetadataRefreshSet(setSize)
}

// TokenPriceRefreshSet pulls s set of NFT tokens scheduled to be updated their price.
func (p *Proxy) TokenPriceRefreshSet(setSize int64) ([]*types.Token, error) {
	return p.db.TokenPriceRefreshSet(setSize)
}

// TokenPriceRefresh recalculates token prices and updates them in database.
func (p *Proxy) TokenPriceRefresh(t *types.Token) error {
	return p.db.TokenPriceRefresh(t)
}

func (p *Proxy) TokenLikesViewsRefresh(t *types.Token) error {
	err := p.LoadTokenLikesViews(t)
	if err != nil {
		return err
	}
	return p.db.TokenLikesViewsStore(t)
}

func (p *Proxy) LoadTokenLikesViews(t *types.Token) error {
	views, err := p.shared.GetTokenViews(t.Contract, big.Int(t.TokenId))
	if err != nil {
		return err
	}
	likes, err := p.shared.GetTokenLikesCount(&t.Contract, (*big.Int)(&t.TokenId))
	if err != nil {
		return err
	}
	t.CachedViews = views.Int64()
	t.CachedLikes = likes
	t.LikesUpdate = types.Time(time.Now())
	return nil
}

func (p *Proxy) TokenLikesViewsRefreshSet(setSize int64) ([]*types.Token, error) {
	return p.db.TokenLikesViewsRefreshSet(setSize)
}

// TokenMarkListed marks the given NFT as listed for direct sale for the given price.
func (p *Proxy) TokenMarkListed(contract *common.Address, tokenID *big.Int, price types.TokenPrice, ts *time.Time) error {
	return p.db.TokenMarkListed(contract, tokenID, price, ts)
}

// TokenMarkOffered marks the given NFT as having offer for the given price.
func (p *Proxy) TokenMarkOffered(contract *common.Address, tokenID *big.Int, price types.TokenPrice, ts *time.Time) error {
	return p.db.TokenMarkOffered(contract, tokenID, price, ts)
}

// TokenMarkAuctioned marks the given NFT as having auction for the given price.
func (p *Proxy) TokenMarkAuctioned(contract *common.Address, tokenID *big.Int, price types.TokenPrice, ts *time.Time) error {
	return p.db.TokenMarkAuctioned(contract, tokenID, price, ts)
}

// TokenMarkBid marks the given NFT as having auction bid for the given price.
func (p *Proxy) TokenMarkBid(contract *common.Address, tokenID *big.Int, price types.TokenPrice, ts *time.Time) error {
	return p.db.TokenMarkBid(contract, tokenID, price, ts)
}

// TokenMarkUnlisted marks the given NFT as not listed for direct sale.
func (p *Proxy) TokenMarkUnlisted(contract *common.Address, tokenID *big.Int) error {
	return p.db.TokenMarkUnlisted(contract, tokenID)
}

// TokenMarkUnOffered marks the given NFT as not having offer anymore.
func (p *Proxy) TokenMarkUnOffered(contract *common.Address, tokenID *big.Int) error {
	return p.db.TokenMarkUnOffered(contract, tokenID)
}

// TokenMarkUnAuctioned marks the given NFT as not auctioned.
func (p *Proxy) TokenMarkUnAuctioned(contract *common.Address, tokenID *big.Int) error {
	return p.db.TokenMarkUnAuctioned(contract, tokenID)
}

// TokenMarkUnBid marks the given NFT as not having a bid anymore.
func (p *Proxy) TokenMarkUnBid(contract *common.Address, tokenID *big.Int) error {
	return p.db.TokenMarkUnBid(contract, tokenID)
}

// TokenMarkSold marks the given NFT as transferred OR sold on a listing/offer/auction for the given price.
func (p *Proxy) TokenMarkSold(contract *common.Address, tokenID *big.Int, price *types.TokenPrice, tradeTime *time.Time) error {
	return p.db.TokenMarkSold(contract, tokenID, price, tradeTime)
}

func (p *Proxy) TokenMarkBanned(contract *common.Address, tokenID *big.Int, banned bool) error {
	return p.db.TokenMarkBanned(contract, tokenID, banned)
}

func (p *Proxy) TokenMarkCollectionBanned(contract *common.Address, banned bool) error {
	return p.db.TokenMarkCollectionBanned(contract, banned)
}

// ListTokens loads a list of tokens from the local database.
// A callback for legacy extension is provided to the loader.
func (p *Proxy) ListTokens(filter *types.TokenFilter, sorting sorting.TokenSorting, sortDesc bool, cursor types.Cursor, count int, backward bool) (*types.TokenList, error) {
	return p.db.ListTokens(filter, sorting, sortDesc, cursor, count, backward)
}

func (p *Proxy) GetTokenJsonMetadata(uri string) (*types.JsonMetadata, error) {
	var key strings.Builder
	key.WriteString("GetTokenJsonMetadata")
	key.WriteString(uri)

	jsonMetadata, err, _ := p.callGroup.Do(key.String(), func() (interface{}, error) {
		return p.uri.GetJsonMetadata(uri)
	})
	return jsonMetadata.(*types.JsonMetadata), err
}

// GetImage downloads an image expected on the given URI.
func (p *Proxy) GetImage(imgUri string) (*types.Image, error) {
	var key strings.Builder
	key.WriteString("GetImage")
	key.WriteString(imgUri)

	data, err, _ := p.callGroup.Do(key.String(), func() (interface{}, error) {
		return p.uri.GetImage(imgUri)
	})
	if err != nil {
		log.Errorf("image can not be loaded from %s; %s", imgUri, err.Error())
		return nil, err
	}
	if nil == data {
		log.Errorf("image not found at %s", imgUri)
		return nil, fmt.Errorf("image not found at given URI")
	}
	return data.(*types.Image), err
}

// GetImageThumbnail generates a thumbnail for an image expected to be downloadable from the given URI.
func (p *Proxy) GetImageThumbnail(imgUri string) (*types.Image, error) {
	var key strings.Builder
	key.WriteString("GetImageThumbnail")
	key.WriteString(imgUri)

	data, err, _ := p.callGroup.Do(key.String(), func() (interface{}, error) {
		image, err := p.GetImage(imgUri)
		if err != nil {
			return nil, fmt.Errorf("image loading failed for %s; %s", imgUri, err)
		}
		if nil == image {
			return nil, fmt.Errorf("image %s not found", imgUri)
		}

		log.Infof("loaded %s of type %s", imgUri, image.Type.Mimetype())
		thumb, err := createThumbnail(*image)
		if err != nil {
			return nil, fmt.Errorf("thumbnail creation failed; %s", err)
		}
		return &thumb, nil
	})
	return data.(*types.Image), err
}

func (p *Proxy) UploadTokenData(metadata types.JsonMetadata, image types.Image) (uri string, err error) {
	return p.pinner.PinTokenData(metadata, image)
}

type royaltyRecipient struct {
	royalty   uint16
	recipient common.Address
}

// GetTokenRoyalty provides fee for token minter when the token is sold and its recipient (royalty has 2 decimals)
func (p *Proxy) GetTokenRoyalty(contract common.Address, tokenId *big.Int) (royalty int32, recipient common.Address, err error) {
	var key strings.Builder
	key.WriteString("GetTokenRoyalty")
	key.WriteString(contract.String())
	key.WriteString(tokenId.String())

	rr, err, _ := p.callGroup.Do(key.String(), func() (interface{}, error) {
		royalty, recipient, err := p.rpc.GetTokenRoyalty(contract, tokenId)
		return royaltyRecipient{royalty, recipient}, err
	})
	return int32(rr.(royaltyRecipient).royalty), rr.(royaltyRecipient).recipient, err
}
