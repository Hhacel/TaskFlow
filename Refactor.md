# Prompt Refaktoryzacyjny: TaskFlow - Skalowalny System Orkiestracji Procesów Biznesowych

**Kontekst i Cel:**
Jako doświadczony inżynier oprogramowania, przeprowadź kompleksową refaktoryzację aktualnego projektu, dostosowując go ściśle do architektury i wymagań prototypowego systemu "TaskFlow", opisanego poniżej. System ten ma pełnić rolę skalowalnego orkiestratora procesów biznesowych (workflowów) opartego na rozproszonej architekturze sterowanej zdarzeniami. 

**Kluczowa zasada:** Możesz wykorzystać obecną implementację jako punkt wyjścia, ale **nie wolno Ci dodawać ani odejmować żadnych funkcjonalności, które nie zostały wprost opisane w tym dokumencie**. System ma realizować wyłącznie poniższe założenia.

---

## 1. Wymagania Technologiczne
*   **Język programowania:** Cały system (Orkiestrator, API Gateway, Workery) musi być zaimplementowany w języku **Go (Golang)**.
*   **Baza Danych:** Wyłącznym trwałym źródłem informacji o stanie workflowów, zadań, zależnościach i rezultatach jest **PostgreSQL**.
*   **Broker Komunikatów:** Komunikacja asynchroniczna pomiędzy Orkiestratorem a Workerami musi odbywać się za pomocą brokera **NATS**.
    *   Wymagane jest użycie mechanizmu *Queue Groups* w NATS, aby zapewnić automatyczne równoważenie obciążenia (load balancing) pomiędzy wieloma instancjami tego samego Workera.
*   **Infrastruktura:** System musi być zaprojektowany pod kątem konteneryzacji (**Docker**) oraz zarządzania i skalowania w **Kubernetes**. (Sam kod powinien pozwalać na niezależne budowanie obrazów dla poszczególnych komponentów).

---

## 2. Model Danych (PostgreSQL)
Struktura bazy danych musi składać się **wyłącznie** z poniższych encji i atrybutów:

1.  **Workflow** (reprezentuje instancję procesu)
    *   `id`: integer (Klucz główny)
    *   `name`: varchar/integer(10)
    *   `status`: varchar(255)
    *   `created_at`: timestamp/integer(10)
    *   `updated_at`: timestamp/integer(10)

2.  **Task** (pojedyncze zadanie w ramach workflowu)
    *   `id`: integer (Klucz główny)
    *   `workflow_id`: integer (Klucz obcy do Workflow)
    *   `name`: varchar/integer(10)
    *   `command`: clob/text (polecenie do wykonania)
    *   `status`: varchar(255)
    *   `created_at`: timestamp/integer(10)
    *   `updated_at`: timestamp/integer(10)
    *   `timeout`: integer(10) (limit czasu na wykonanie, wartość może być Null/pusta)

3.  **TaskDependency** (określa graf zależności (DAG) dla zadań)
    *   `task_id`: integer (Klucz obcy do Task - identyfikator zadania zależnego)
    *   `depends_on_task_id`: integer (Klucz obcy do Task - identyfikator zadania, które musi się zakończyć, aby `task_id` mogło wystartować)
    *   *(Klucz główny kompozytowy z obu tych pól)*

4.  **TaskResult** (rezultat wykonania konkretnej próby zadania)
    *   `id`: integer (Klucz główny)
    *   `task_id`: integer (Klucz obcy do Task)
    *   `attempt`: integer(10) (numer próby wykonania)
    *   `success`: integer(10)/boolean (informacja o powodzeniu)
    *   `output`: clob/text (dane wyjściowe, nullable)
    *   `error`: clob/text (komunikat o błędzie, nullable)
    *   `start_time`: timestamp/integer(10)
    *   `end_time`: timestamp/integer(10)

---

## 3. Architektura Komponentów (Mikrousługi)
Zastosuj asynchroniczną komunikację i odizoluj system na następujące, niezależne komponenty (kontenery):

*   **API Gateway:**
    *   Wystawia interfejs REST API dla klienta.
    *   Przyjmuje żądania HTTP i przekazuje je do Orkiestratora.
*   **Orchestrator (Orkiestrator):**
    *   Główny silnik logiczny (autorski silnik workflow).
    *   Analizuje zależności między zadaniami (z tabeli `TaskDependency`).
    *   Decyduje, które zadania są gotowe do wykonania, i publikuje polecenia ich wykonania do brokera NATS (określone `subjecty`).
    *   Nasłuchuje w NATS na komunikaty z wynikami (od Workerów) i na ich podstawie aktualizuje stany w bazie PostgreSQL oraz decyduje o kontynuacji procesu.
    *   Implementacja NATS powinna być oparta na uniwersalnym interfejsie (warstwie abstrakcji), aby uniezależnić projekt na poziomie kodu.
*   **Worker(s):**
    *   Warstwa wykonawcza.
    *   Instancje nasłuchują na NATS (w modelu Queue Groups).
    *   Pobierają polecenia (zadania), wykonują je i publikują wyniki (powodzenie, dane wyjściowe lub błąd) z powrotem do NATS.
    *   Są całkowicie bezstanowe i odizolowane od bazy danych Orkiestratora. Można je horyzontalnie skalować.

---

## 4. Automaty Stanowe (Cykl Życia)
Zaimplementuj rygorystyczne przejścia stanów dla Workflow i Task.

### Stany Workflow:
*   `CREATED`: Definicja została utworzona i zapisana w bazie.
*   `RUNNING`: Workflow został uruchomiony. Orkiestrator zaczyna planować zadania. Workflow pozostaje w tym stanie, dopóki są oczekujące lub działające zadania.
*   `COMPLETED`: **Wszystkie** zadania workflowu zakończyły się pomyślnie.
*   `FAILED`: Dowolne zadanie zakończyło się błędem i nie ma możliwości jego ponowienia. Cały proces kończy się niepowodzeniem.
*   `CANCELLED`: Proces został przerwany przez użytkownika. Planowanie nowych zadań zostaje powstrzymane.

### Stany Task:
*   `PENDING`: Zadanie utworzone, oczekuje na spełnienie zależności.
*   `RUNNING`: Zależności zostały spełnione, polecenie wysłane do NATS, zadanie jest (lub za chwilę będzie) przetwarzane przez Workera.
*   `SUCCEEDED`: Worker zwrócił informację o pomyślnym wykonaniu.
*   `FAILED`: Worker zwrócił błąd **LUB** przekroczono zdefiniowany dla zadania `timeout`.
    *   *Uwaga dotycząca ponawiania (Retry):* Jeśli zadanie ma status `FAILED`, system może zadecydować o ponownym wykonaniu (przejście z powrotem do `RUNNING`), np. jeśli określony `timeout` wciąż na to pozwala lub polityka workflowu to zakłada. Wymaga to inkrementacji pola `attempt` w `TaskResult`.
*   `CANCELLED`: Wymuszone przerwanie (gdy cały workflow zostanie anulowany, zadania `PENDING` i `RUNNING` zmieniają status na `CANCELLED`).

---

## 5. Przypadki Użycia (Interfejs REST API)
System musi realizować **tylko i wyłącznie** 5 poniższych przypadków użycia za pośrednictwem API:

1.  **Create Workflow (`PU-001`)**
    *   **Akcja:** Zwalidowanie przesłanej definicji (struktura zadań, zależności) i zapisanie w bazie PostgreSQL.
    *   **Skutek:** Utworzenie rekordu Workflow ze statusem `CREATED`, rekordów Task i TaskDependency.
    *   **Odpowiedź:** Identyfikator (ID) utworzonego zasobu.
2.  **Start Workflow (`PU-002`)**
    *   **Akcja:** Zmiana statusu z `CREATED` na `RUNNING`.
    *   **Skutek:** Orkiestrator odczytuje zadania niezależne (bez warunków wstępnych) i publikuje je natychmiast do NATS.
    *   **Odpowiedź:** System asynchronicznie zwraca potwierdzenie przyjęcia żądania (kod HTTP `202 Accepted`).
3.  **Get Workflow Status (`PU-003`)**
    *   **Akcja:** Pobranie danych o stanie na podstawie ID.
    *   **Skutek:** Orkiestrator odczytuje bazę danych.
    *   **Odpowiedź:** Skonsolidowany raport: główny stan workflowu oraz stany wszystkich jego zadań.
4.  **Get Task Results (`PU-004`)**
    *   **Akcja:** Żądanie pobrania wygenerowanych rezultatów ukończonego zadania (np. status, logi wyjściowe).
    *   **Skutek:** System weryfikuje dostępność danych w tabeli `TaskResult`.
    *   **Odpowiedź:** Zwrócenie ładunku danych (output/error) powiązanego z wykonanym zadaniem.
5.  **Cancel Workflow (`PU-005`)**
    *   **Akcja:** Natychmiastowe przerwanie przetwarzania dla bieżącego ID.
    *   **Skutek:** Zmiana stanu workflow na `CANCELLED`. Zadania oczekujące i w toku otrzymują sygnał przerwania / zmianę statusu na `CANCELLED`. Zaprzestanie wysyłania zadań do kolejki.

---

**Podsumowanie dla modelu językowego przeprowadzającego refaktoryzację:**
Twój kod musi ściśle odzwierciedlać podział ról, strukturę relacyjnej bazy danych, asynchroniczną naturę Orkiestratora reagującego na Eventy z NATS oraz maszyny stanów przedstawione powyżej. Pozbądź się wszelkiego kodu niezgodnego z tym dokumentem.