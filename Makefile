.DEFAULT_GOAL := help

.PHONY: help build up restart down logs

help:
	@printf '%s\n' 'make build    Build the news-app image' 'make up       Start the app' 'make restart  Rebuild and recreate the app' 'make down     Stop the app' 'make logs     Follow app logs'

build:
	docker compose build news-app

up:
	docker compose up -d

 :
	docker compose up -d --build --force-recreate

down:
	docker compose down

logs:
	docker compose logs -f news-app
