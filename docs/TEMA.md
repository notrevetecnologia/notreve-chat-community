# Tema visual

O painel usa uma linguagem visual clara e operacional: fundo frio quase branco, superfícies brancas, verde como ação primária e índigo como cor de apoio. O objetivo é manter leitura rápida em atendimento, sem competir com o conteúdo das conversas.

## Princípios

- Hierarquia por espaçamento, peso tipográfico e badges; não depender só de cor.
- Estados de hover, foco visível e `prefers-reduced-motion` para acessibilidade.
- Cartões e painéis com borda suave e sombra curta, evitando aparência pesada.
- Layout responsivo: a navegação compacta e o atendimento passam a empilhar em telas menores.
- IDs, `data-*`, classes de mensagens e funções globais do `app.js` permanecem compatíveis com os fluxos atuais.

## Tokens principais

As variáveis ficam no início e no refinamento final de `web/static/style.css`: `--bg`, `--card`, `--border`, `--text`, `--muted`, `--accent`, `--accent-dark`, `--indigo`, `--shadow` e `--shadow-hover`. Novas telas devem reutilizar esses tokens em vez de introduzir cores isoladas.
