package scenario

import (
	"fmt"
	"time"

	"github.com/isucon/isucon9-qualify/bench/asset"
	"github.com/isucon/isucon9-qualify/bench/fails"
	"github.com/isucon/isucon9-qualify/bench/session"
)

const (
	CorrectCardNumber = "AAAAAAAA"
	FailedCardNumber  = "FA10AAAA"
	IsucariShopID     = "11"

	ItemStatusOnSale  = "on_sale"
	ItemStatusTrading = "trading"
	ItemStatusSoldOut = "sold_out"
	ItemStatusStop    = "stop"
	ItemStatusCancel  = "cancel"

	ItemsPerPage = 48
)

func sellAndBuy(user1, user2 asset.AppUser) error {
	s1, err := session.NewSession()
	if err != nil {
		return err
	}

	s2, err := session.NewSession()
	if err != nil {
		return err
	}

	seller, err := s1.Login(user1.AccountName, user1.Password)
	if err != nil {
		return err
	}

	if !user1.Equal(seller) {
		return fails.NewError(nil, "ログインが失敗しています")
	}

	err = s1.SetSettings()
	if err != nil {
		return err
	}

	buyer, err := s2.Login(user2.AccountName, user2.Password)
	if err != nil {
		return err
	}

	if !user2.Equal(buyer) {
		return fails.NewError(nil, "ログインが失敗しています")
	}

	err = s2.SetSettings()
	if err != nil {
		return err
	}

	targetItemID, err := s1.Sell("abcd", 100, "description description", 32)
	if err != nil {
		return err
	}
	token, err := s2.PaymentCard(CorrectCardNumber, IsucariShopID)
	if err != nil {
		return err
	}
	_, err = s2.Buy(targetItemID, token)
	if err != nil {
		return err
	}

	apath, err := s1.Ship(targetItemID)
	if err != nil {
		return err
	}

	surl, err := s1.DecodeQRURL(apath)
	if err != nil {
		return err
	}

	s3, err := session.NewSession()
	if err != nil {
		return err
	}

	err = s3.ShipmentAccept(surl)
	if err != nil {
		return err
	}

	err = s1.ShipDone(targetItemID)
	if err != nil {
		return err
	}

	<-time.After(6 * time.Second)

	err = s2.Complete(targetItemID)
	if err != nil {
		return err
	}

	return nil
}

func bumpAndNewItems(user1, user2 asset.AppUser) error {
	s1, err := session.NewSession()
	if err != nil {
		return err
	}

	s2, err := session.NewSession()
	if err != nil {
		return err
	}

	seller, err := s1.Login(user1.AccountName, user1.Password)
	if err != nil {
		return err
	}

	if !user1.Equal(seller) {
		return fails.NewError(nil, "ログインが失敗しています")
	}

	err = s1.SetSettings()
	if err != nil {
		return err
	}

	buyer, err := s2.Login(user2.AccountName, user2.Password)
	if err != nil {
		return err
	}

	if !user2.Equal(buyer) {
		return fails.NewError(nil, "ログインが失敗しています")
	}

	err = s2.SetSettings()
	if err != nil {
		return err
	}

	targetItemID := asset.GetUserItemsFirst(user1.ID)
	newCreatedAt, err := s1.Bump(targetItemID)
	if err != nil {
		return err
	}

	asset.SetItemCreatedAt(user1.ID, targetItemID, newCreatedAt)

	hasNext, items, err := s2.NewItems()
	if err != nil {
		return err
	}

	if !hasNext {
		return fails.NewError(nil, "/new_items.jsonのhas_nextがfalseです")
	}

	if len(items) != ItemsPerPage-1 {
		return fails.NewError(nil, fmt.Sprintf("/new_items.jsonの商品数が違います: expected: %d; actual: %d", ItemsPerPage-1, len(items)))
	}

	// 簡易チェック
	createdAt := items[0].CreatedAt
	found := false
	for _, item := range items {
		if createdAt < item.CreatedAt {
			return fails.NewError(nil, "/new_items.jsonはcreated_at順である必要があります")
		}

		if item.Status != ItemStatusOnSale && item.Status != ItemStatusSoldOut {
			return fails.NewError(nil, "/new_items.jsonは販売中か売り切れの商品しか出してはいけません")
		}

		aItem, ok := asset.GetItem(item.SellerID, item.ID)
		if ok && !(aItem.Name == item.Name && aItem.Price == item.Price && aItem.Status == item.Status) {
			// TODO: aItem.CreatedAt == item.CreatedAtはinitializeを実装しないと確認できない
			return fails.NewError(nil, "/new_items.jsonで返している商品の情報に誤りがあります")
		}

		if targetItemID == item.ID {
			found = true
		}

		createdAt = item.CreatedAt
	}

	if !found {
		// Verifyでしかできない確認
		return fails.NewError(nil, "/new_items.jsonにバンプした商品が表示されていません")
	}

	targetItemID, targetItemCreatedAt := items[len(items)-1].ID, items[len(items)-1].CreatedAt

	hasNext, items, err = s2.NewItemsWithItemIDAndCreatedAt(targetItemID, targetItemCreatedAt)
	if err != nil {
		return err
	}

	if !hasNext {
		return fails.NewError(nil, "/new_items.jsonのhas_nextがfalseです")
	}

	if len(items) != ItemsPerPage-1 {
		return fails.NewError(nil, fmt.Sprintf("/new_items.jsonの商品数が違います: expected: %d; actual: %d", ItemsPerPage-1, len(items)))
	}

	createdAt = items[0].CreatedAt
	for _, item := range items {
		if !(item.ID < targetItemID && item.CreatedAt <= targetItemCreatedAt) {
			return fails.NewError(nil, "/new_items.jsonのitem_idとcreated_atが正しく動作していません")
		}

		if createdAt < item.CreatedAt {
			return fails.NewError(nil, "/new_items.jsonはcreated_at順である必要があります")
		}

		if item.Status != ItemStatusOnSale && item.Status != ItemStatusSoldOut {
			return fails.NewError(nil, "/new_items.jsonは販売中か売り切れの商品しか出してはいけません")
		}

		aItem, ok := asset.GetItem(item.SellerID, item.ID)
		if ok && !(aItem.Name == item.Name && aItem.Price == item.Price && aItem.Status == item.Status) {
			// TODO: aItem.CreatedAt == item.CreatedAtはinitializeを実装しないと確認できない
			return fails.NewError(nil, "/new_items.jsonで返している商品の情報に誤りがあります")
		}

		createdAt = item.CreatedAt
	}

	return nil
}

func newCategoryItems(user1 asset.AppUser) error {
	s1, err := session.NewSession()
	if err != nil {
		return err
	}

	seller, err := s1.Login(user1.AccountName, user1.Password)
	if err != nil {
		return err
	}

	if !user1.Equal(seller) {
		return fails.NewError(nil, "ログインが失敗しています")
	}

	err = s1.SetSettings()
	if err != nil {
		return err
	}

	category := asset.GetRandomRootCategory()

	hasNext, rootCategoryName, items, err := s1.NewCategoryItems(category.ID)
	if err != nil {
		return err
	}

	if !hasNext {
		return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonのhas_nextがfalseです", category.ID))
	}

	if len(items) != ItemsPerPage-1 {
		return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonの商品数が違います: expected: %d; actual: %d", category.ID, ItemsPerPage-1, len(items)))
	}

	if rootCategoryName != category.CategoryName {
		return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonのカテゴリ名が間違えています", category.ID))
	}

	// 簡易チェック
	createdAt := items[0].CreatedAt
	for _, item := range items {
		if createdAt < item.CreatedAt {
			return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonはcreated_at順である必要があります", category.ID))
		}

		if item.Status != ItemStatusOnSale && item.Status != ItemStatusSoldOut {
			return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonは販売中か売り切れの商品しか出してはいけません", category.ID))
		}

		aItem, ok := asset.GetItem(item.SellerID, item.ID)
		if ok && !(aItem.Name == item.Name && aItem.Price == item.Price && aItem.Status == item.Status) {
			// TODO: aItem.CreatedAt == item.CreatedAtはinitializeを実装しないと確認できない
			return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonで返している商品の情報に誤りがあります", category.ID))
		}

		createdAt = item.CreatedAt
	}

	targetItemID, targetItemCreatedAt := items[len(items)-1].ID, items[len(items)-1].CreatedAt

	hasNext, rootCategoryName, items, err = s1.NewCategoryItemsWithItemIDAndCreatedAt(category.ID, targetItemID, targetItemCreatedAt)
	if err != nil {
		return err
	}

	if !hasNext {
		return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonのhas_nextがfalseです", category.ID))
	}

	if len(items) != ItemsPerPage-1 {
		return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonの商品数が違います: expected: %d; actual: %d", category.ID, ItemsPerPage-1, len(items)))
	}

	if rootCategoryName != category.CategoryName {
		return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonのカテゴリ名が間違えています", category.ID))
	}

	// 簡易チェック
	createdAt = items[0].CreatedAt
	for _, item := range items {
		if !(item.ID < targetItemID && item.CreatedAt <= targetItemCreatedAt) {
			return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonのitem_idとcreated_atが正しく動作していません", category.ID))
		}

		if createdAt < item.CreatedAt {
			return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonはcreated_at順である必要があります", category.ID))
		}

		if item.Status != ItemStatusOnSale && item.Status != ItemStatusSoldOut {
			return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonは販売中か売り切れの商品しか出してはいけません", category.ID))
		}

		aItem, ok := asset.GetItem(item.SellerID, item.ID)
		if ok && !(aItem.Name == item.Name && aItem.Price == item.Price && aItem.Status == item.Status) {
			// TODO: aItem.CreatedAt == item.CreatedAtはinitializeを実装しないと確認できない
			return fails.NewError(nil, fmt.Sprintf("/new_items/%d.jsonで返している商品の情報に誤りがあります", category.ID))
		}

		createdAt = item.CreatedAt
	}

	return nil
}