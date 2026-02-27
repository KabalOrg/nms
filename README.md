# ⚡ NMS Kabal: Система моніторингу електроживлення та вузлів

Це проста та ефективна система моніторингу мережевих вузлів (NMS), написана на **Go**. Вона дозволяє відстежувати стан напруги, наявність 220V на об'єктах та отримувати сповіщення в Telegram. Проект орієнтований на роботу з пристроями по протоколу **SNMP** (наприклад, Equicom, D-Link тощо) та Deye Cloud API.

## 🚀 Основні можливості

* **Моніторинг SNMP:** Збір даних про напругу (V) та статус мережі через OID.
* **Інтеграція з Deye Cloud:** Моніторинг гібридних інверторів Deye через офіційне API.
* **Telegram Бот:** * Сповіщення про зникнення та появу світла.
    * Сповіщення про запуск/зупинку генераторів.
    * Перегляд логів подій у реальному часі.
    * Обмеження доступу (тільки для авторизованих користувачів).
* **Веб-інтерфейс (Vue.js + Tailwind):**
    * Сучасний Dashboard з групуванням по містах/районах.
    * Візуалізація стану батарей (LiFePO4 / AGM).
    * Адмін-панель для додавання та редагування вузлів прямо з браузера.
* **Гнучкість:** Підтримка різних типів живлення (пряме, від клієнтів, через UPS).

## 🛠 Технологічний стек

* **Backend:** Go (Golang)
* **Frontend:** Vue.js 3, Tailwind CSS
* **Протоколи:** SNMP, HTTP API
* **База даних:** JSON-файл (для легкого перенесення та бекапу)

## 📋 Встановлення та запуск

1.  **Клонуйте репозиторій:**
    ```bash
    git clone [https://github.com/ТВОЙ_USERNAME/nms-kabal.git](https://github.com/ТВОЙ_USERNAME/nms-kabal.git)
    cd nms-kabal
    ```

2.  **Налаштуйте змінні середовища:**
    Скопіюйте файл приклад та введіть свої дані (токен бота, пароль адміна):
    ```bash
    cp .env.example .env
    nano .env
    ```

3.  **Завантажте залежності Go:**
    ```bash
    go mod download
    ```

4.  **Запустіть проект:**
    ```bash
    go run main.go
    ```
    Після цього веб-інтерфейс буде доступний на порту, вказаному в коді (за замовчуванням `8085`).

## ⚙️ Налаштування .env

Обов'язково заповніть наступні поля:
* `BOT_TOKEN` — токен від @BotFather.
* `ADMIN_PASSWORD` — пароль для входу в адмін-панель.
* `ALLOWED_USERS` — ID користувачів Telegram (через кому), які зможуть користуватися ботом.

## ⚙️ Налаштування Deye Cloud

Для роботи з інверторами Deye необхідно додати облікові дані в файл `.env`. 

⚠️ **Важливо:** З міркувань безпеки API Deye вимагає пароль у форматі **SHA256**. 

Перетворити свій пароль у SHA256 можна в терміналі Linux:
```bash
echo -n "ваш_пароль" | sha256sum
```
Отриманий хеш вставте в поле DEYE_PASSWORD у файлі .env.

📄 Конфігурація .env
Обов'язково заповніть:

BOT_TOKEN — токен від @BotFather.

ALLOWED_USERS — ID користувачів Telegram для доступу.

DEYE_EMAIL — логін (email) від аккаунта Deye Cloud.

DEYE_PASSWORD — пароль у форматі SHA256.


## 📸 Скриншоти
<img width="1914" height="842" alt="gitscreen2" src="https://github.com/user-attachments/assets/cd1b1db9-4694-46b0-8eec-1f2681164371" />
<img width="1887" height="416" alt="gitscreen" src="https://github.com/user-attachments/assets/2856827b-cdd9-4753-88f9-6cb04ac7a3f1" />
<img width="1895" height="928" alt="gitscreen3" src="https://github.com/user-attachments/assets/fa45e763-6134-4854-9c62-0cfc5700537b" />


## 🍕 Підтримати проект (Donate)

Якщо ця система допомогла вашому бізнесу або провайдеру пережити відключення світла, ви можете підтримати розвиток проекту:

<a href="https://donatello.to/kabal_org" target="_blank">
  <img src="https://img.shields.io/badge/Підтримати_на-Donatello-FF5722?style=for-the-badge" alt="Donatello">
</a>


<a href="https://send.monobank.ua/jar/Abc4m6jPBC" target="_blank">
  <img src="https://img.shields.io/badge/Прямий_донат-Monobank-000000?style=for-the-badge" alt="Monobank">
</a>



## 📄 Ліцензія
Цей проект розповсюджується під ліцензією MIT.



