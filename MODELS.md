# Gemini Models & Thinking Levels Catalog

Guida e catalogo ufficiale di tutti i modelli, livelli di reasoning e alias verificati in `custom-free-gemini-api`.

---

## 🧭 I Modelli Reali di Google Gemini Web

Dall'interfaccia web di `gemini.google.com` (verificata da cattura del traffico HAR e interfaccia utente), sono disponibili esattamente **4 modelli attivi**:

| Modello Web | API Model ID | ModeID / Hash | Livello Default | Thinking Levels Supportati | Note & Ruolo |
| :--- | :--- | :--- | :---: | :--- | :--- |
| **4 Neon** (*Frontier adattivo*) | `gemini-4-neon` | `"neon"` / `e6fa609c3fa255c0` | `medium` (4) | `minimal` (0), `low` (1), `medium` (4), `high` (2), `xhigh` (8) | 🌟 Catturato dal HAR `net-log-gemini-neon.har` (2026-11-21): family `3`, thinking adattivo multi-livello. |
| **4 Argon** (*Codice e analisi*) | `gemini-4-argon` | `"argon"` / `95a4c0abcba24364` | `medium` (4) | `low` (1), `medium` (4), `high` (2) | 🧠 Ragionamento frontier per compiti complessi, codice e analisi. |
| **4 Hydrogen** (*Aiuto quotidiano*) | `gemini-4-hydrogen` | `56fdd199312815e2` | `high` (2) | `low` (1), `medium` (4), `high` (2), `xhigh` (8) | ⚡ Modello rapido quotidiano con reasoning avanzato e supporto ricerca. |
| **3.5 Flash-Lite** (*Risposte rapide*) | `gemini-3.5-flash-lite` | `8c46e95b1a07cecc` | `minimal` (0) | `minimal` (0), `low` (1), `medium` (4), `high` (2) | 🚀 Risposte istantanee con minima latenza. |
| **3.1 Pro** (*Ricerca dettagliata*) | `gemini-3.1-pro` | `e6fa609c3fa255c0` | `low` (1) | `low` (1), `medium` (1), `high` (2) | 💎 Estratto da HAR reale: family index `3`, thinking standard (`1`) e alto (`2`). |
| **4 Carbon** (*Attività complesse*) | — | *Non disponibile* | — | — | 🔒 Esclusivo per abbonati **Ultra** (bloccato su account Pro/Free). |
| **Test Fake 1** | `gemini-fake-model-1` | `aaaaaaaaaaaaaaaa` | `medium` | Test diagnostico | 🧪 Genera intenzionalmente il fallback e l'avviso rosso. |
| **Test Fake 2** | `gemini-fake-model-2` | `gemini-6-nonexisting`| `medium` | Test diagnostico | 🧪 Genera intenzionalmente il fallback e l'avviso rosso. |

> ℹ️ **Nota su Gemini Pro & Thinking**:
> Nel web Google per Pro sono disponibili due livelli: standard/normale (`1`) e alto (`2`). Il livello intermedio `medium` viene mappato su `1` per evitare anomalie (`{"37":[0]}`).

> ℹ️ **Nota sugli alias storici di Flash (3.8, 3.7, 3.6, 3.5)**:
> Nell'interfaccia web di Google non esistono selettori separati per "3.8 Flash" o "3.7 Flash". Se richiedi `gemini-3.8-flash` o l'alias `flash`, la richiesta viene instradata direttamente sul motore di *Aiuto quotidiano* (`gemini-4-hydrogen`, hash `56fdd199312815e2`).

---

## 🧠 Mapping del Protocollo JSPB (`x-goog-ext-525001261-jspb`)

Ogni famiglia di modelli richiede una struttura specifica per non incorrere nel fallback di Google:

* **Neon & Pro**: `idx 14 = 3`, `idx 15 = <livello>`, capabilities `[4,33,22,5,6,8,16,19,20,38]`
* **Argon & Hydrogen & Flash**: `idx 14 = 1`, `idx 15 = <livello>`, capabilities `[4,33,22,5,6,8,16,19,20,38]`
* **Flash-Lite**: `idx 14 = 6`, `idx 15 = <livello>`

### Codici dei Livelli di Thinking
| Nome Parametro | Codice JSPB | Descrizione |
| :--- | :---: | :--- |
| `minimal` | `0` | Zero token di riflessione. Risposta immediata (default Flash-Lite). |
| `low` | `1` | Riflessione rapida e leggera. |
| `medium` | `4` | Riflessione bilanciata (default Argon). |
| `high` | `2` | Ragionamento approfondito (default Hydrogen). |
| `xhigh` | `8` | Massima deliberazione (esclusivo Hydrogen). |

---

## 📌 Alias Supportati per Chiamate API

### 1. Gemini 4 Neon
- `gemini-4-neon`, `neon`, `4-neon`, `gemini-neon` *(default: medium)*
- `gemini-4-neon-thinking-minimal`, `gemini-4-neon-minimal`, `neon-minimal` *(minimal: 0)*
- `gemini-4-neon-thinking-low`, `gemini-4-neon-low`, `neon-low` *(low: 1)*
- `gemini-4-neon-thinking-medium`, `gemini-4-neon-medium`, `neon-medium` *(medium: 4)*
- `gemini-4-neon-thinking-high`, `gemini-4-neon-high`, `neon-high` *(high: 2)*
- `gemini-4-neon-thinking-xhigh`, `gemini-4-neon-xhigh`, `neon-xhigh` *(xhigh: 8)*

### 2. Gemini 4 Argon
- `gemini-4-argon`, `argon`, `4-argon`, `gemini-4` *(default: medium)*
- `gemini-4-argon-thinking-low`, `gemini-4-argon-low`, `argon-low`, `thinking-low`
- `gemini-4-argon-thinking-medium`, `gemini-4-argon-medium`, `argon-medium`, `thinking-medium`
- `gemini-4-argon-thinking-high`, `gemini-4-argon-high`, `argon-high`, `thinking-high`

### 2. Gemini 4 Hydrogen
- `gemini-4-hydrogen`, `hydrogen`, `4-hydrogen` *(default: high)*
- `gemini-4-hydrogen-thinking-low`, `gemini-4-hydrogen-low`, `hydrogen-low`
- `gemini-4-hydrogen-thinking-medium`, `gemini-4-hydrogen-medium`, `hydrogen-medium`
- `gemini-4-hydrogen-thinking-high`, `gemini-4-hydrogen-high`, `hydrogen-high`
- `gemini-4-hydrogen-thinking-xhigh`, `gemini-4-hydrogen-xhigh`, `hydrogen-xhigh`

### 3. Gemini 3.5 Flash-Lite
- `gemini-3.5-flash-lite`, `flash-lite`, `gemini-flash-lite`, `lite` *(default: minimal)*
- `gemini-3.5-flash-lite-thinking-minimal`, `flash-lite-minimal`
- `gemini-3.5-flash-lite-thinking-low`, `flash-lite-low`
- `gemini-3.5-flash-lite-thinking-medium`, `flash-lite-medium`
- `gemini-3.5-flash-lite-thinking-high`, `flash-lite-high`

### 4. Gemini 3.1 Pro
- `gemini-3.1-pro`, `pro`, `3.1-pro`, `gemini-pro`, `advanced` *(default: low/standard)*
- `gemini-3.1-pro-thinking-high`, `pro-thinking-high`, `pro-high`
- `gemini-3.1-pro-thinking-medium`, `pro-thinking-medium`, `pro-medium`
- `gemini-3.1-pro-thinking-low`, `pro-thinking-low`, `pro-low`

### 5. Alias Storici Flash & Thinking
- `gemini-3.8-flash`, `flash`, `default` (con varianti `-thinking-low`, `-thinking-medium`, `-thinking-high`)
- `gemini-3.7-flash` (con varianti `-thinking-low`, `-thinking-medium`, `-thinking-high`)
- `gemini-3.6-flash` (con varianti `-thinking-minimal`, `-thinking-low`, `-thinking-medium`, `-thinking-high`)
- `gemini-3.5-flash` (con varianti `-thinking-minimal`, `-thinking-low`, `-thinking-medium`, `-thinking-high`)
- `gemini-3.8-thinking`, `flash-thinking`, `thinking` *(deep reasoning con scratchpad trace)*

---

## 🛡️ Affidabilità del Rilevamento Fallback

### Quando sei sicuro al 100% che il banner scatta:
1. **Hash / Model ID inventati o non riconosciuti** (es. `gemini-fake-model-1`, `fake-2`): Google restituisce il flag di protocollo `{"37":[0]}` o un modello incongruente in `inner[42]`, facendo scattare immediatamente il box rosso.
2. **Downgrade palesi di tier** (es. richiedere `gemini-3.1-pro` con cookie di account Free non abbonato): l'account non ha i permessi e Google risponde con Flash.
3. **Downgrade da Argon a Flash-Lite**: se la sessione perde l'autenticazione o scade, il mismatch è totale e il banner scatta al 100%.

### Dove NON c'è la certezza al 100% (I 3 casi limite):
1. **Il caso Hydrogen vs Flash (stesso hash `56fdd199312815e2`)**: condividendo lo stesso ModeID di base a livello infrastrutturale, la distinzione risiede unicamente nella profondità di reasoning JSPB; se Google eseguisse Flash anziché Hydrogen senza cambiare l'etichetta interna, non scatta il banner.
2. **Downgrade parziale del solo Thinking Level**: se chiedi Hydrogen o Argon a livello `high` o `xhigh`, ma Google per congestione riduce internamente i token di riflessione mantenendo la stessa famiglia, il nome del modello in `inner[42]` resta invariato.
3. **Modifiche non annunciate al protocollo JSPB**: se Google dovesse spostare il nome del modello dall'indice `inner[42]` senza inviare il pacchetto `{"37":[0]}`, in assenza di un campo popolato il sistema non ha evidenza per attivare l'allarme.
