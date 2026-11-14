UPDATE mensagens
SET texto = BTRIM(REPLACE(REPLACE(texto, ' ' || chr(128522), ''), chr(128522), ''))
WHERE texto LIKE '%' || chr(128522) || '%';

UPDATE channels
SET widget_config = REPLACE(widget_config, '"icone_botao":"' || chr(128522) || '"', '"icone_botao":""')
WHERE widget_config LIKE '%"icone_botao":"' || chr(128522) || '"%';
