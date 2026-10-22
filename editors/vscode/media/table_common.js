// Tablo görünümleri için ortak mantık (table.js ve watch_panel.js'den önce yüklenir): sayı mı,
// Türkçe büyük/küçük harfe duyarsız süzme ve sütuna göre sıralama. DOM kurma her görünümün kendisinindir.
const isNumber = (x) => x !== '' && x !== null && !isNaN(Number(x));

// tableRows rows'u (satır dizileri) query'yi içeren satırlara süzer ve sortState'e ({column, descending};
// column < 0: sıralama yok) göre sıralar. Sayısal hücreler sayı olarak, öbürleri Türkçe sıraya göre karşılaştırılır.
function tableRows(rows, query, sortState) {
  const q = query.toLocaleLowerCase('tr');
  let list = rows.filter((r) => !q || r.some((h) => String(h).toLocaleLowerCase('tr').includes(q)));
  if (sortState.column >= 0) {
    const k = sortState.column;
    list = list.slice().sort((a, b) => {
      const c = isNumber(a[k]) && isNumber(b[k]) ? Number(a[k]) - Number(b[k]) : String(a[k]).localeCompare(String(b[k]), 'tr');
      return sortState.descending ? -c : c;
    });
  }
  return list;
}
