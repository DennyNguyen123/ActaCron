package manager

import (
	"os"
	"path/filepath"
)

var defaultSharedFiles = map[string]string{
	"datetime.js": `/**
 * Shared Datetime Helpers
 */
function nowISO() {
    return new Date().toISOString();
}

function formatDate(date, formatStr) {
    var d = date ? new Date(date) : new Date();
    var pad = function(n) { return n < 10 ? '0' + n : '' + n; };
    var YYYY = d.getFullYear();
    var MM = pad(d.getMonth() + 1);
    var DD = pad(d.getDate());
    var HH = pad(d.getHours());
    var mm = pad(d.getMinutes());
    var ss = pad(d.getSeconds());

    if (!formatStr) return YYYY + '-' + MM + '-' + DD + ' ' + HH + ':' + mm + ':' + ss;
    return formatStr
        .replace('YYYY', YYYY)
        .replace('MM', MM)
        .replace('DD', DD)
        .replace('HH', HH)
        .replace('mm', mm)
        .replace('ss', ss);
}

function timeAgo(date) {
    var diffMs = Date.now() - new Date(date).getTime();
    var sec = Math.floor(diffMs / 1000);
    if (sec < 60) return sec + 's ago';
    var min = Math.floor(sec / 60);
    if (min < 60) return min + 'm ago';
    var hr = Math.floor(min / 60);
    if (hr < 24) return hr + 'h ago';
    var days = Math.floor(hr / 24);
    return days + 'd ago';
}

function addDays(date, n) {
    var d = new Date(date);
    d.setDate(d.getDate() + n);
    return d;
}

function startOfDay(date) {
    var d = date ? new Date(date) : new Date();
    d.setHours(0, 0, 0, 0);
    return d;
}

module.exports = {
    nowISO: nowISO,
    formatDate: formatDate,
    timeAgo: timeAgo,
    addDays: addDays,
    startOfDay: startOfDay
};
`,
	"notify.js": `/**
 * Shared Notification & Webhook Helpers
 */
function telegram(opts) {
    opts = opts || {};
    var token = opts.botToken || (typeof env === 'function' ? env('TELEGRAM_BOT_TOKEN') : '');
    var chatId = opts.chatId || (typeof env === 'function' ? env('TELEGRAM_CHAT_ID') : '');
    var message = opts.message || '';

    if (!token || !chatId) {
        throw new Error('Telegram botToken and chatId are required');
    }

    var url = 'https://api.telegram.org/bot' + token + '/sendMessage';
    return http.post(url, {
        chat_id: chatId,
        text: message,
        parse_mode: opts.parseMode || 'HTML'
    }, { 'Content-Type': 'application/json' });
}

function discord(opts) {
    opts = opts || {};
    var webhookUrl = opts.webhookUrl || (typeof env === 'function' ? env('DISCORD_WEBHOOK_URL') : '');
    if (!webhookUrl) {
        throw new Error('Discord webhookUrl is required');
    }
    var payload = {};
    if (opts.content) payload.content = opts.content;
    if (opts.embeds) payload.embeds = opts.embeds;

    return http.post(webhookUrl, payload, { 'Content-Type': 'application/json' });
}

function slack(opts) {
    opts = opts || {};
    var webhookUrl = opts.webhookUrl || (typeof env === 'function' ? env('SLACK_WEBHOOK_URL') : '');
    if (!webhookUrl) {
        throw new Error('Slack webhookUrl is required');
    }
    return http.post(webhookUrl, { text: opts.text || '' }, { 'Content-Type': 'application/json' });
}

function webhook(url, payload, headers) {
    headers = headers || { 'Content-Type': 'application/json' };
    return http.post(url, payload, headers);
}

module.exports = {
    telegram: telegram,
    discord: discord,
    slack: slack,
    webhook: webhook
};
`,
	"utils.js": `/**
 * Shared General Utility Helpers
 */
function uuid() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        var r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

function retry(fn, opts) {
    opts = opts || {};
    var retries = opts.retries || 3;
    var delayMs = opts.delayMs || 1000;
    var backoff = opts.backoff || 2;

    var currentDelay = delayMs;
    for (var i = 0; i < retries; i++) {
        try {
            return fn(i);
        } catch (err) {
            if (i === retries - 1) throw err;
            if (typeof sleep === 'function') {
                sleep(currentDelay);
            }
            currentDelay *= backoff;
        }
    }
}

function chunk(arr, size) {
    if (!arr || !size || size <= 0) return [];
    var res = [];
    for (var i = 0; i < arr.length; i += size) {
        res.push(arr.slice(i, i + size));
    }
    return res;
}

function safeJson(str, defaultValue) {
    try {
        return JSON.parse(str);
    } catch (e) {
        return defaultValue !== undefined ? defaultValue : null;
    }
}

function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    var k = 1024;
    var sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatCurrency(amount, currency) {
    currency = currency || 'VND';
    var num = Number(amount) || 0;
    return num.toLocaleString('vi-VN') + ' ' + currency;
}

module.exports = {
    uuid: uuid,
    retry: retry,
    chunk: chunk,
    safeJson: safeJson,
    formatBytes: formatBytes,
    formatCurrency: formatCurrency
};
`,
	"constants.js": `/**
 * Shared System Constants
 */
module.exports = {
    HTTP_STATUS: {
        OK: 200,
        CREATED: 201,
        NO_CONTENT: 204,
        BAD_REQUEST: 400,
        UNAUTHORIZED: 401,
        FORBIDDEN: 403,
        NOT_FOUND: 404,
        SERVER_ERROR: 500
    },
    REGEX: {
        EMAIL: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
        URL: "^https?:\\/\\/.+$",
        PHONE_VN: "^(0|\\+84)[3|5|7|8|9][0-9]{8}$"
    },
    TIME: {
        SECOND_MS: 1000,
        MINUTE_MS: 60000,
        HOUR_MS: 3600000,
        DAY_MS: 86400000
    }
};
`,
	"index.js": `/**
 * Shared Library Aggregate Exports
 */
var datetime = require('./datetime');
var notify = require('./notify');
var utils = require('./utils');
var constants = require('./constants');

module.exports = {
    datetime: datetime,
    notify: notify,
    utils: utils,
    constants: constants
};
`,
}

// ScaffoldSharedLibrary checks and creates default shared library files in sharedDir
func ScaffoldSharedLibrary(sharedDir string) error {
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		return err
	}

	for filename, content := range defaultSharedFiles {
		filePath := filepath.Join(sharedDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
