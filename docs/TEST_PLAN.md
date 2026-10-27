# Roteiro Completo de Testes (Test Plan) — Cyberspace Refactor

Este documento descreve a rotina completa de verificação automatizada e manual para validar o **Cyberspace Refactor** no Windows 10 e Windows 11.

---

## Parte 1 — Bateria Automatizada (Unitários, Corpus Legado, Fuzzing, Segurança e Build)

### 1.1 Execução com um Único Comando (`scripts\verify.ps1`)
Abra o **PowerShell** na pasta raiz do projeto e execute:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ".\Cyberspace Refactor\scripts\verify.ps1"
```

**O que o script verifica automaticamente:**
1. **`go vet ./...`**: Análise estática de todos os pacotes (`cmd/cyberspace`, `internal/app`, `internal/assets`, `internal/dice`, `internal/engine`, `internal/format`, `internal/model`, `internal/safefs`, `internal/ui`).
2. **Testes Unitários e de Segurança (`go test -v ./... -count=1`)**:
   - `internal/dice`: Regra do 6 explosivo (`Rule of Six`), limite anti-travamento (`MaxExplosions = 50`) e cálculo de sucessos.
   - `internal/safefs`: Bloqueio de Path Traversal (`../`), nomes reservados do Windows (`CON`, `NUL`, `COM1`), Alternate Data Streams (`:` no nome), escrita atômica e criação de `.bak`.
   - `internal/model`: Ordenação horária de saídas a partir das 6 horas (`Exits`) e `Clone()` profundo.
   - `internal/format`: Parsing e round-trip de **todo o corpus legado de 241 arquivos** (`.ltg`, `.dek`, `.npc`) + serialização `.json` + reparo automático de links (`FixLinks`).
   - `internal/engine`: 25 cenários cobrindo alcances (`Observation` -> `Sensor` -> `Contact`), estados de ICE, utilitários (`Analyze`, `Attack`, `Slow`, `Deception`, `Sleaze`, `Medic`, `Restore`, `Relocate`, `Browse`, `Evaluate`, `Decrypt`, `Armor`, `Cloak`, `Shield`, `Mirrors`, `Smoke`, `Compressor`), todas as 16 Operações de Sistema, contagem regressiva de `Trace`, proteção de satélite, modos de Hacking Pool e transição entre matrizes (`LNK`).
   - `internal/assets`: Carregamento real de `.bmp` e `.wav`, transparência de fundo branco (`#FFFFFF`) nos sprites de ICE e rejeição de BMPs com dimensões maliciosas (`> 2048×2048`).
   - `internal/app`: Isolamento de logs `.txt` dentro de `assets/LOGS/` (impedindo sobrescrita de aventuras `.txt` em `assets/DATA/`).
   - `internal/security_test.go`: Verificação estática garantindo **zero imports de pacotes de rede** (`net`, `net/http`, etc.) e proteção contra loop infinito em nós `LNK` cíclicos.
3. **Fuzzing Rápido (`go test -fuzz=... -fuzztime=2s`)**:
   - Executa `FuzzParseLTG`, `FuzzParseDeck` e `FuzzParseNPC` contra entradas mutadas aleatoriamente.
4. **Compilação e Auto-Teste do Executável (`--headless-check`)**:
   - Compila `Cyberspace Refactor.exe` e executa `.\Cyberspace Refactor.exe --headless-check`, que carrega `FIRSTRUN.LTG`, `FUCHI4_2.DEK` e `HEAVYHIT.NPC`, inicia uma sessão real na Matrix e avança um turno.

---

## Parte 2 — Roteiro de Testes Manuais (Passo a Passo na Interface Gráfica)

Para iniciar o aplicativo, dê dois cliques em **`Cyberspace Refactor\Cyberspace Refactor.exe`** (ou execute `& ".\Cyberspace Refactor\Cyberspace Refactor.exe"` no PowerShell).

### Caso de Teste M1 — Inicialização Rápida e Navegação na Matrix (`Cyberspace Terminal`)
1. Na aba **1. Cyberspace Terminal**, clique no botão verde **"Quick start (First Run + Fuchi Cyber-4 + Heavy Hitter)"**.
   - **Resultado esperado**: A tela principal do terminal é aberta. No **Vidscreen** aparece a imagem do nó `SAN` em alcance `Observation` (`SAN2.BMP`), no **Matrix map** à direita aparece o nó `#0` destacado com um anel branco, e o histórico registra a entrada na Matrix.
2. Clique em **Forward** (ou pressione a tecla `Seta para Cima` / `W`).
   - **Resultado esperado**: O alcance muda de `Observation` para `Sensor` (`SAN1.BMP`). O ICE `Access` no SAN pede identificação (`wants ID`) ou é exibido na lista **ICE in Node**.
3. Selecione `Sleaze` ou `Deception` na lista **Active Memory** (ou carregue de **Storage Memory** clicando no programa e em **Load**) e clique em **Exec** (com o ICE selecionado em **ICE in Node**).
   - **Resultado esperado**: O teste de utilitário é rolado no **History Log** e o ICE muda para `[sleazed]` ou `[deceived]`.
4. Clique em **Forward** novamente para entrar em alcance `Contact` (`SAN0.BMP`), e mais uma vez em **Forward** para avançar pela linha de dados até o próximo nó (`#1`).
   - **Resultado esperado**: O nó `#1` é revelado no mapa com névoa de guerra (`fog of war`) atualizada e suas saídas conectadas aparecem como `?` até serem analisadas/visitadas.

### Caso de Teste M2 — Operações de Sistema, Download de Arquivos e Exportação de Log
1. Em qualquer nó em alcance **Contact** (`Range 0`), observe o painel **System Operations**:
   - **Resultado esperado**: Os botões mudam dinamicamente conforme o tipo de nó (`SAN`, `SPU`, `DS`, `SM`, `IOP`, `CPU`).
2. Em um nó **DS (Datastore)** em alcance **Contact**, execute o utilitário `Browse` ou `Evaluate` (ou clique nas operações de sistema `Directory` / `Read` / `Transfer`).
   - **Resultado esperado**: Os arquivos do Datastore aparecem na lista **Datastore Files**; ao selecionar um arquivo e clicar em **Transfer**, ele é baixado para a **Storage Memory** do cyberdeck.
3. Clique em **Save Log** no painel esquerdo e confirme o caminho `LOGS/run.txt`.
   - **Resultado esperado**: A barra de status confirma `Log saved to LOGS/run.txt` e o arquivo `Cyberspace Refactor\assets\LOGS\run.txt` é criado sem tocar em nenhum `.txt` dentro de `DATA/`.

### Caso de Teste M3 — Editor Visual de Matrizes (`Matrix Construction Set - MATED`)
1. Clique na aba superior **2. Matrix Construction Set**.
2. Clique em **Open .LTG/.JSON**, filtre por `FIRSTRUN.LTG` e clique em **Open**.
   - **Resultado esperado**: Toda a topologia da matriz `First Run` é desenhada no grid central com cores, formatos geométricos (`SAN` retângulo, `CPU` hexágono duplo, `DS` quadrado, `IOP` triângulo, `SPU` hexágono) e linhas de dados verdes.
3. Com **Mode: Select / Create Node** ativo, clique em uma célula vazia do grid.
   - **Resultado esperado**: Um novo nó é criado naquela posição e conectado ao nó anteriormente selecionado.
4. No painel direito (**Node Inspector**), altere o tipo para `DS`, a cor para `Orange`, o Rating para `5`, adicione um arquivo (`+Add File`) e um ICE (`+ICE`).
5. Clique em **Validate Rules** na barra superior.
   - **Resultado esperado**: O painel **Consistency & Rule Validator** exibe o resultado da validação.
6. Clique em **Save .LTG** e salve como `DATA/TEST_MATED.LTG`, depois clique em **Export .JSON** e salve como `JSON/TEST_MATED.json`.
   - **Resultado esperado**: Ambos os arquivos são salvos com escrita atômica e podem ser reabertos tanto no `MATED` quanto no `Cyberspace Terminal`.

### Caso de Teste M4 — Construtor de Cyberdeck (`Cyberdeck Designer - CYBDECK`)
1. Clique na aba superior **3. Cyberdeck Designer**.
2. Clique em **Open .DEK/.NPC/.JSON**, selecione `DATA/ALLEGIANCE.DEK` (ou `DATA/FUCHI4_2.DEK`) e clique em **Open**.
   - **Resultado esperado**: Todos os atributos de Persona (`MPCP`, `Bod`, `Evasion`, `Masking`, `Sensor`), Hardware e lista de utilitários são carregados, e o painel **Cyberdeck Design Rule Check** mostra a validação de pontos de Persona e memória.
3. Clique em **Preset:** para escolher um programa padrão (ex.: `Shield`), clique em **+Add Utility** e depois em **Load Into Terminal**.
   - **Resultado esperado**: A barra de status informa que o cyberdeck projetado foi carregado diretamente para a próxima corrida no Terminal.

### Caso de Teste M5 — Biomonitor de Personagem (`Decker Biomonitor - DECKERED`)
1. Clique na aba superior **4. Decker Biomonitor**.
2. Clique em **Open .NPC/.JSON**, selecione `DATA/HEAVYHIT.NPC` e clique em **Open**.
   - **Resultado esperado**: Os atributos naturais, perícias, reservas de dados e o preview ao vivo de **Matrix Initiative** e **Hacking Pool** (nas duas fórmulas: Original `CYBERSPC.EXE` e `SR2 Core`) são exibidos.
3. Altere `Computer Skill` ou `Intelligence`, clique em **Apply Decker Biomonitor** e verifique a atualização imediata no painel **Derived Matrix Biomonitor Preview**.

### Caso de Teste M6 — Configurações, Áudio e Conversão em Lote para JSON (`Settings & About`)
1. Clique na aba superior **5. Settings & About**.
2. Clique em **Test Sound (SNDBOOT.WAV)**.
   - **Resultado esperado**: O efeito sonoro original de boot (`SNDBOOT.WAV`) é reproduzido via `winmm.dll`.
3. Alterne a fórmula de Hacking Pool ou o modo de exibição e clique em **Save Settings to settings.json**.
4. Clique em **Convert All Legacy Files to assets/JSON/**.
   - **Resultado esperado**: Os arquivos `.ltg`, `.dek` e `.npc` válidos em `assets/DATA/` são convertidos para JSON dentro de `assets/JSON/` sem modificar nenhum arquivo original.
