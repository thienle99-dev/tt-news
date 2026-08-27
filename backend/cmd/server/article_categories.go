package main

import (
	"context"
)

func (s *server) replaceArticleCategories(ctx context.Context, articleID int64, definitions []categoryDefinition) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM article_categories WHERE article_id=?", articleID); err != nil {
		return err
	}
	for _, definition := range definitions {
		if _, err = tx.ExecContext(ctx, "INSERT INTO categories(slug,name) VALUES(?,?) ON CONFLICT(slug) DO NOTHING", definition.slug, definition.name); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO article_categories(article_id,category_id) SELECT ?,id FROM categories WHERE slug=?`, articleID, definition.slug); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *server) attachArticleCategories(ctx context.Context, item *article) error {
	rows, err := s.db.QueryContext(ctx, `SELECT c.slug,c.name FROM article_categories ac JOIN categories c ON c.id=ac.category_id WHERE ac.article_id=? ORDER BY ac.category_id`, item.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	item.Categories = nil
	for rows.Next() {
		var category articleCategory
		if err = rows.Scan(&category.Slug, &category.Name); err != nil {
			return err
		}
		item.Categories = append(item.Categories, category)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(item.Categories) == 0 && item.Category != "" {
		item.Categories = []articleCategory{{Slug: item.Category, Name: item.Category}}
	}
	return nil
}

func (s *server) attachArticleCategoriesList(ctx context.Context, items []article) error {
	for index := range items {
		if err := s.attachArticleCategories(ctx, &items[index]); err != nil {
			return err
		}
	}
	return nil
}
