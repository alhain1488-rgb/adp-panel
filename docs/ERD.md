# ERD — модель данных

Диаграмма сущностей из `SPEC.md §4`. Протокол-специфичные параметры хранятся в
JSON-полях (`settings_json`, `stream_settings_json`, `sniffing_json`) — добавление
протокола (включая Hysteria2) не требует изменения схемы.

```mermaid
erDiagram
    admins ||--o{ audit_logs : "performs"
    servers ||--o{ inbounds : "hosts"
    clients ||--o{ client_inbounds : "granted"
    inbounds ||--o{ client_inbounds : "grants"

    admins {
        int id PK
        string username
        string password_hash
        string totp_secret_enc
        bool totp_enabled
        datetime created_at
        datetime updated_at
    }
    servers {
        int id PK
        string name
        string host
        int ssh_port
        string ssh_user
        string ssh_auth_method
        string ssh_secret_enc
        string ssh_passphrase_enc
        string xray_config_path
        string xray_service_name
        string hysteria_config_path
        string hysteria_service_name
        string ip
        string geo_country
        string geo_city
        string geo_asn
        string status
        string provision_status
        string provision_error
        datetime last_check_at
        datetime last_sync_at
        string last_sync_error
        datetime created_at
        datetime updated_at
    }
    inbounds {
        int id PK
        int server_id FK
        string tag
        string protocol
        string listen
        int port
        json settings_json
        json stream_settings_json
        json sniffing_json
        string remark
        bool enabled
        datetime created_at
        datetime updated_at
    }
    clients {
        int id PK
        string name
        string uuid
        string password
        string subscription_token
        bool enabled
        string remark
        datetime created_at
        datetime updated_at
    }
    client_inbounds {
        int id PK
        int client_id FK
        int inbound_id FK
        bool enabled
        datetime created_at
    }
    audit_logs {
        int id PK
        int admin_id FK
        string action
        string target_type
        int target_id
        json detail_json
        string ip
        string user_agent
        datetime created_at
    }
    settings {
        string key PK
        string value
    }
```

## Ключевые ограничения

- `client_inbounds`: уникальность `(client_id, inbound_id)`; `enabled` — точечное
  выключение доступа без удаления связи.
- `clients.subscription_token` — уникальный неугадываемый токен подписки.
- `clients.uuid` — для протоколов на UUID (VLESS/VMess); `clients.password` — для
  протоколов на пароле (Trojan/Shadowsocks/Hysteria2). Оба поля генерируются при
  создании клиента и стабильны.
- `inbounds.protocol` — строка; целевой движок определяется через реестр протоколов,
  а не отдельной колонкой.
- `settings` — key-value: домен/базовый URL подписки, интервал синхронизации, выбранный
  движок Hysteria2 и т.п.
- `servers.provision_status` (`pending`/`installing`/`installed`/`failed`) и `provision_error` —
  состояние автоустановки движков на ноду (SPEC §5.1). Server-level поля, добавлены аддитивной
  миграцией; sync не пушит на сервер, пока не `installed`.
