.PHONY: compose-up compose-down

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down
