// Kod şablonları kişinin yazım dilinde sunulur: şablon dosyası (snippets/snippets.json) her yazım dili
// için ayrı bir liste taşır ve `go run ./tools/gen/snippets` ile üretilir; elle düzenlenmez. Kısayol, ad
// ve durakların varsayılan sözcükleri yazım dilindedir; gövde her dilde kimlik yazımıdır (TR_if): şablon
// kodlu dosyaya yazılır, kod yalnız kimliklerle yazılır (17.7). VS Code'un sabit şablon katkısı (contributes.snippets) dil seçemediği için şablonlar
// tamamlama önerisi olarak verilir: öneri, ayarın (tr.writingLanguage) o anki diline göre
// seçilir, Tab ile durak durak doldurulur. vscode modülüne yalnızca register bağlıdır; böylece
// seçim kuralı Node ile ayrıca sınanır (test/snippets_test.js).
'use strict';

const fs = require('fs');
const path = require('path');

const DEFAULT_LANGUAGE = 'tr';

// loadTable şablon dosyasını okur: { dil: [{ label, description, prefix: [...], body: [...] }] }.
function loadTable(extensionPath) {
    return JSON.parse(fs.readFileSync(path.join(extensionPath, 'snippets', 'snippets.json'), 'utf8'));
}

// completions lang dilinin şablonlarını öneri satırlarına açar (her kısayol bir satır); dilin listesi
// yoksa Türkçe kullanılır.
function completions(table, lang) {
    const templates = table[lang] || table[DEFAULT_LANGUAGE] || [];
    return templates.flatMap((t) => t.prefix.map((prefix) => ({
        prefix, label: t.label, description: t.description, snippet: t.body.join('\n')
    })));
}

// register yazım diline göre şablon önerilerini .tr belgelerine ekler. Metot şablonlarının ".ad"
// kısayolu, imlecin solunda nokta varsa noktayı da değiştirir (liste.fil → liste.TR_filter(...)).
function register(vscode, table, writingLanguage) {
    return vscode.languages.registerCompletionItemProvider({ language: 'tr' }, {
        provideCompletionItems(document, position) {
            const word = document.getWordRangeAtPosition(position);
            const dot = word && word.start.character > 0
                && document.getText(new vscode.Range(word.start.translate(0, -1), word.start)) === '.';
            return completions(table, writingLanguage()).map((c) => {
                const item = new vscode.CompletionItem(c.prefix, vscode.CompletionItemKind.Snippet);
                item.detail = c.label;
                item.documentation = c.description;
                item.insertText = new vscode.SnippetString(c.snippet);
                if (c.prefix.startsWith('.') && dot) {
                    item.range = new vscode.Range(word.start.translate(0, -1), position);
                }
                return item;
            });
        }
    });
}

module.exports = { loadTable, completions, register };
