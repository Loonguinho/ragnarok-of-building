<div align="center">

<img src="docs/img/icon.png" width="96" alt="">

# Ragnarok of Building

**Monte sua build de Ragnarok Online LATAM, veja o dano e descubra o que melhorar.**

[![Versão](https://img.shields.io/github/v/release/ragnarok-of-building/rob?include_prereleases&label=vers%C3%A3o&color=3f6fb5)](../../releases)
[![Downloads](https://img.shields.io/github/downloads/ragnarok-of-building/rob/total?label=downloads&color=3f6fb5)](../../releases)
[![Testes](https://img.shields.io/github/actions/workflow/status/ragnarok-of-building/rob/ci.yml?branch=main&label=testes)](../../actions)
[![Licença MIT](https://img.shields.io/badge/licen%C3%A7a-MIT-lightgrey)](LICENSE)

### [▶️ Abrir no navegador](https://ragnarok-of-building.github.io/rob/) · [⬇️ Baixar para Windows](../../releases)

Português · English · Español

</div>

![Tela de equipamentos](docs/img/equipamento.png)

## O que ele faz

- **Build completa:** atributos, talentos, árvore de habilidades, equipamentos, cartas, encantamentos, refino, grau, sombras, visuais, munição e pet.
- **Dano contra qualquer monstro do catálogo:** normal, crítico, chance de acerto e usos para matar, com a tabela das suas habilidades.
- **Importa replay (`.rrf`):** lê o personagem gravado e resume o dano da gravação.
- **Otimizador:** testa itens, cartas e encantamentos e mostra o ganho de cada troca.
- **Melhor carta:** ordena as opções de uma posição por dano ou defesa, contando conjuntos.
- **Comparações:** itens lado a lado e distribuições de talentos.
- **Compartilhar:** link, código ou paste para mandar a um amigo.
- **Dados do LATAM:** itens, textos e habilidades vêm do cliente LATAM.

| Cálculo de dano | Comparar talentos |
|---|---|
| ![Cálculo de dano](docs/img/calculos.png) | ![Comparar talentos](docs/img/comparar-talentos.png) |

<details>
<summary>Árvore de habilidades</summary>

![Árvore de habilidades](docs/img/habilidades.png)

</details>

## Como usar

**No navegador:** abra [ragnarok-of-building.github.io/rob/](https://ragnarok-of-building.github.io/rob/). Não precisa instalar nada; os builds ficam guardados neste navegador. Para mandar um build a alguém, use **Compartilhar → Link**.

**No Windows:**

1. Baixe o `.zip` da [última versão](../../releases) e extraia em qualquer pasta.
2. Abra o `RagnarokOfBuilding.exe`.
3. Escolha sua classe ou clique em **Importar replay**.

Não precisa instalar nada nem ter o jogo: os dados já vêm no zip. Os ícones são baixados uma vez, quando aparecem pela primeira vez.

> O programa não é assinado, então o Windows pode mostrar um aviso: **Mais informações → Executar assim mesmo**. O código está todo aqui para quem quiser conferir.

Mais detalhes no [LEIAME](docs/LEIAME.txt). Aperte **Alt+H** no programa para ver os atalhos.

## O dano confere com o jogo?

As fórmulas vêm do latam-ro-calc, do bROWiki e do rAthena, e são conferidas com replays do servidor LATAM. Algumas habilidades já batem com a gravação; outras ainda precisam de mais dados. É uma versão **beta**.

Achou um dano errado? [Abra uma issue](../../issues) com o **replay (`.rrf`)** e um print dos **atributos (Alt+A)** com os talentos.

## Para quem programa

Feito em Go, com janela nativa em [Gio](https://gioui.org). Como compilar, importar os dados, regenerar as fórmulas e publicar uma versão: [docs/DESENVOLVIMENTO.md](docs/DESENVOLVIMENTO.md) (em inglês).

## Créditos

- **[latam-ro-calc](https://github.com/adsonpleal/latam-ro-calc)** e **[rrfparser](https://github.com/adsonpleal/rrfparser)**, de Adson Leal (MIT): fórmulas e monstros do LATAM, e referência do formato de replay.
- **[bROWiki](https://browiki.org)**: tabelas das habilidades, usadas para conferir as fórmulas.
- **[rAthena](https://github.com/rathena/rathena)** (GPL-3.0): tabelas de classes, itens e monstros.
- **[Divine Pride](https://www.divine-pride.net)**: ícones e atributos de alguns monstros.

O que vem de cada fonte está detalhado em [docs/DESENVOLVIMENTO.md](docs/DESENVOLVIMENTO.md#credits).

## Licença

O código está sob a [licença MIT](LICENSE). Os dados e as imagens do jogo não fazem parte dela: Ragnarok Online é © Gravity Co., Ltd. Este é um projeto de fãs, sem ligação com a Gravity.
