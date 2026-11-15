---
name: gemini-mode-catalog
description: Guida operativa per comprendere i codici di stato (OK, Fallback, Anomaly 37, Timeout), testare i modelli con Go e curl, estrarre Mode ID da catture HAR di Google Gemini Web e aggiungere nuovi modelli al catalogo.
---

# Guida Operativa Gemini Mode ID & Catalogo Modelli

Questa skill documenta l'architettura interna di routing, i parametri JSPB di Google Gemini Web, le modalità di testing (Go CLI e curl), la diagnosi degli errori/fallback e la procedura esatta per intercettare e aggiungere nuovi modelli a `custom-free-gemini-api`.

---

## 1. Architettura del Protocollo Google Gemini Web

`custom-free-gemini-api` inoltra le richieste ricevute sugli endpoint standard OpenAI (`/v1/chat/completions`) all'endpoint interno di Google Gemini Web (`StreamGenerateContent` / `assistant.vertical`).

L'instradamento verso il modello corretto avviene tramite l'header HTTP:
```http
x-goog-ext-525001261-jspb: [1,null,null,null,"<MODE_ID>",null,"it",null,[CAPABILITIES],null,null,null,null,null,<FAMILY_IDX>,<THINKING_LVL>,...]
```

### Struttura dell'Array JSPB
| Indice Array | Parametro | Descrizione & Valori Verificati |
| :---: | :--- | :--- |
| **`[0]`** | Versione Protocollo | Sempre `1`. |
| **`[4]`** | **ModeID** | Identificatore del modello: hash esadecimale a 16 caratteri (es. `"e6fa609c3fa255c0"`, `"56fdd199312815e2"`, `"8c46e95b1a07cecc"`) oppure stringa letterale (es. `"argon"`). |
| **`[6]`** | Lingua | Locale client (es. `"it"`, `"en"` o `null`). |
| **`[8]`** | **Capabilities** | Array di feature supportate: standard `[4,33,22,5,6,8,16,19,20,38]` per modelli completi/thinking; `[4]` per Flash-Lite ultra-rapido. |
| **`[14]`** | **Family Index** | **Fondamentale**: indica a Google quale motore instradare.<br>• `1`: Famiglia Argon / Hydrogen / Flash<br>• `3`: Famiglia Gemini 3.1 Pro *(estratto da HAR live)*<br>• `6`: Famiglia Flash-Lite<br>*(⚠️ Attenzione: il vecchio valore `8` è deprecato da Google e causa timeout di 120s)*. |
| **`[15]`** | **Thinking Level** | Profondità del chain-of-thought:<br>• `0` = *Minimal* (0 token thinking, istantaneo)<br>• `1` = *Low / Standard* (ragionamento leggero, default Pro)<br>• `4` = *Medium* (bilanciato; valido per Argon/Hydrogen/Flash, **non** supportato da Pro)<br>• `2` = *High* (massima deliberazione; valido per Argon, Hydrogen, Flash e Pro)<br>• `8` = *XHigh* (deliberazione estrema, esclusivo Hydrogen). |

---

## 2. Codici di Stato, Fallback & Diagnosi Errori

Durante i test o l'uso dell'API, le risposte possono trovarsi nei seguenti stati:

### `✅ 200 OK` (Successo Diretto)
* **Significato**: Il modello ha risposto e l'header di risposta `X-Actual-Model` corrisponde alla famiglia richiesta.
* **Header**: `X-Actual-Model: 3.1 Pro`, `X-Actual-Model: 4 Hydrogen`, ecc.

### `⚠️ FALLBACK / DOWNGRADE` (Retrocessione Silenziosa di Google)
* **Significato**: Google ha accettato la connessione, ma ha sostituito internamente il modello richiesto con un modello inferiore o generico.
* **Come viene rilevato nel codice**:
  1. **Anomalia 37 (`{"37": [0]}`)**: presente nel payload grezzo di Google quando il `ModeID` non esiste o il `ThinkingLevel` non è ammesso per quel modello (es. livello 4 su Gemini Pro).
  2. **Mismatch su `inner[42]`**: Google restituisce esplicitamente un nome modello diverso (es. richiesta `gemini-4-argon`, ma risposta da `4 Carbon` o `3.5 Flash-Lite`).
  3. **Guest Worker Mismatch**: se una richiesta per un modello che richiede login (`MinTier: "auth"`, come Pro o Argon) viene assegnata al worker `guest`, Google risponde silenziosamente con Flash-Lite.
* **Indicatori**:
  * Console server: banner rosso `[FALLBACK / DOWNGRADE WARNING] DETECTED IN GOOGLE UPSTREAM`.
  * Header HTTP: `X-Fallback-Detected: true`.

### `❌ ERROR - Timeout (120s) / Connessione Fallita`
* **Causa tipica**: Utilizzo di un `idx 14` (Family Index) errato o deprecato nel template JSPB (es. `idx 14 = 8` anziché `1` o `3`). Google non chiude la connessione ma non invia dati, facendo scadere il timeout HTTP di 120s.

### `❌ ERROR - HTTP 500 (immediato a ~0.6s)`
* **Causa tipica**: Cookie account scaduti o non validi. Il worker tenta la richiesta, Google restituisce un redirect a `ServiceLogin`, il worker va in errore e ricarica i session token a ripetizione.
* **Risoluzione**: Eliminare il file cookie scaduto in `cookies/account_<ID>.json` o riautenticarsi tramite l'estensione Chrome.

### `🔒 HTTP 403 (Subscription Required)`
* **Significato**: Il modello ha `MinTier: "subscription"` ma nel pool non è registrato alcun worker con abbonamento Google One AI Premium attivo.

### `🔒 HTTP 401 (Authentication Required)`
* **Significato**: Il modello ha `MinTier: "auth"` (es. Pro o Argon) ma nel pool è presente solo il worker anonimo `guest` (nessun cookie account configurato).

---

## 3. Come Usare lo Script di Test Go (`test_all_models.go`)

Lo script `test_all_models.go` interroga tutti i modelli del catalogo `DefaultBuiltinModels` in parallelo, mostrando una tabella terminale live auto-aggiornante con tempi e risposte.

### Esecuzione Base
```bash
cd /Users/sor-team/Documents/gw2api/custom-free-gemini-api/free-gemini-api
go run test_all_models.go
```

### Parametri & Flag Utili
```bash
# Cambiare il prompt di test (default: "Rispondi in una parola quale è la capitale della Francia")
go run test_all_models.go -prompt="Scrivi una poesia di tre parole sul mare"

# Modificare la concorrenza (default: 4; usa 1 o 2 per evitare rate limit su account singolo)
go run test_all_models.go -concurrency=2

# Puntare a un'istanza remota o su porta diversa
go run test_all_models.go -server="http://127.0.0.1:8001"
```

### Lettura della Tabella Live
* **MODE ID**: l'hash o la stringa inviata a Google.
* **STATUS**: `In attesa`, `Running ..`, `✅ 200 OK`, `⚠️ FALLBACK`, `❌ ERROR`.
* **TEMPO**: durata della chiamata (es. `4.2s`).
* **RISPOSTA / STATO**:
  * Se OK: mostra l'anteprima del testo (es. `Parigi`).
  * Se Fallback: mostra il modello effettivo tra parentesi (es. `[4 Hydro...] Parigi`).
  * Se Errore: mostra il codice HTTP o `Timeout / Connessione fallita`.

---

## 4. Come Testare un Singolo Modello con `curl`

### 1. Chiamata Standard (Non-Streaming)
```bash
curl -i -s -X POST http://127.0.0.1:8001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-3.1-pro",
    "messages": [
      {"role": "user", "content": "Rispondi in una parola quale è la capitale della Francia"}
    ]
  }'
```

### 2. Chiamata Streaming (SSE)
```bash
curl -N -i -X POST http://127.0.0.1:8001/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-4-argon",
    "stream": true,
    "messages": [
      {"role": "user", "content": "Conta da 1 a 5"}
    ]
  }'
```

### 3. Come Ispezionare gli Header di Risposta
Nel testo restituito da `curl -i`, osservare:
* `X-Actual-Model`: Indica il modello Google effettivo (es. `3.1 Pro`, `4 Argon`, `4 Hydrogen`).
* `X-Fallback-Detected: true`: Se presente, segnala che la richiesta è stata retrocessa.

---

## 5. Come Intercettare Nuovi Mode ID da Google Gemini Web (Metodo HAR)

Quando Google lancia un nuovo modello (es. una nuova versione di Pro, Flash o Ultra), per integrarlo è necessario estrarne i parametri reali di rete.

### Procedura di Cattura:
1. Aprire **Google Chrome** e andare su `https://gemini.google.com/app`.
2. Aprire gli **Strumenti per Sviluppatori** (`F12` o `Cmd+Option+I`) e posizionarsi sulla scheda **Network**.
3. Nella casella di filtro, digitare `StreamGenerateContent` (oppure `assistant.vertical` o `f.req`).
4. Nell'interfaccia di Gemini, selezionare il nuovo modello dal menu a tendina e inviare un messaggio qualsiasi (es. "Ciao").
5. Fare clic con il tasto destro sulla richiesta intercettata e selezionare **Save all as HAR with content** salvando il file (es. `network-log.har`).

### Script Python di Estrazione Rapida
Eseguire questo script puntando al file `.har`:

```python
import json

with open("network-log.har", "r", encoding="utf-8") as f:
    har = json.load(f)

for entry in har["log"]["entries"]:
    req = entry["request"]
    for header in req.get("headers", []):
        if "525001261" in header["name"]:
            print(f"URL: {req['url']}")
            print(f"JSPB Header:\n{header['value']}\n")
```

### Parametri da Annotare dall'Header Estratto:
1. **ModeID**: elemento `[4]` (es. `"e6fa609c3fa255c0"` o `"argon"`).
2. **Capabilities**: elemento `[8]` (es. `[4,33,22,5,6,8,16,19,20,38]`).
3. **Family Index**: elemento `[14]` (es. `1`, `3` o `6`).
4. **Thinking Level**: elemento `[15]` (es. `0`, `1`, `2`, `4`).

---

## 6. Procedura per Aggiungere un Nuovo Modello al Codice

Una volta estratti i parametri, per registrare il modello nel server:

### Passo 1: Aggiungere la configurazione in `gemini/registry.go`
Aprire `gemini/registry.go` e aggiungere la struct in `DefaultBuiltinModels`:

```go
{
    ID:             "gemini-nuovo-modello",
    DisplayName:    "Google Gemini Nuovo Modello",
    Description:    "Descrizione del modello e capacità",
    ModeID:         "hash_estratto_da_har",
    HeaderTemplate: `[1,null,null,null,"%s",null,null,null,[4,33,22,5,6,8,16,19,20,38],null,null,null,null,null,3,%d,null,null,null,null]`,
    ThinkingLevel:  1, // Livello default supportato
    MinTier:        "auth", // "free", "auth" (se richiede login) o "subscription"
    Aliases:        []string{"nuovo-modello", "gemini-nuovo"},
    Capabilities:   []string{"chat", "stream", "thinking", "tools"},
    IsDefault:      false,
    Source:         "builtin",
},
```

### Passo 2: Aggiornare la Logica di Downgrade in `gemini/downgrade.go`
In `gemini/downgrade.go`, estendere `IsModelDowngraded`:

```go
if strings.Contains(req, "nuovo-modello") {
    return !strings.Contains(act, "nuovo-modello") && !strings.Contains(act, "parola_chiave_upstream")
}
```

### Passo 3: Aggiornare `MODELS.md`
Aggiungere il nuovo modello nella tabella riepilogativa e documentare alias e codici JSPB.

### Passo 4: Verificare e Riavviare
```bash
# Eseguire i test unitari
go test ./...

# Riavviare il server
killall -9 goapi 2>/dev/null
go run main.go --temp &

# Validare con lo script di test
go run test_all_models.go
```
