/* NabungYuk shared frontend helpers */
window.escapeHTML = function(value) {
  return String(value ?? '').replace(/[&<>'"]/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[ch]));
};

window.renderSidebar = function(activePage) {
  const links = [
    ['dashboard', '/dashboard.html', 'Dashboard'],
    ['transactions', '/transactions.html', 'Transaksi'],
    ['savings', '/savings.html', 'Tabungan'],
    ['reminders', '/reminders.html', 'Pengingat'],
    ['reports', '/reports.html', 'Laporan']
  ];
  const sidebarHtml = `
    <aside class="hidden md:flex md:flex-col md:w-64 bg-white shadow-lg">
      <div class="p-6 border-b"><div class="flex items-center"><span class="text-3xl mr-2">🌱</span><span class="font-bold text-xl text-gray-800">NabungYuk</span></div></div>
      <nav class="flex-1 p-4 space-y-1">
        ${links.map(([key, href, label]) => `<a href="${href}" class="sidebar-link ${activePage === key ? 'sidebar-link-active' : 'sidebar-link-inactive'}">${activePage === key ? '<span class="sidebar-indicator"></span>' : ''}${label}</a>`).join('')}
      </nav>
      <div class="p-4 border-t"><button onclick="logout()" class="sidebar-link sidebar-link-inactive">Keluar</button></div>
    </aside>`;
  const appDiv = document.getElementById('app');
  if (appDiv && !appDiv.querySelector('aside')) appDiv.insertAdjacentHTML('afterbegin', sidebarHtml);
};

window.checkAuth = function() {
  const token = localStorage.getItem('token');
  if (!token) {
    window.location.replace('/index.html');
    return null;
  }
  return token;
};

window.api = async function(url, opts = {}) {
  const token = localStorage.getItem('token');
  const headers = new Headers(opts.headers || {});
  if (token) headers.set('Authorization', `Bearer ${token}`);
  const hasBody = opts.body !== undefined && opts.body !== null;
  const isFormData = typeof FormData !== 'undefined' && opts.body instanceof FormData;
  if (hasBody && !isFormData && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');

  let res;
  try {
    res = await fetch(url, {...opts, headers});
  } catch (error) {
    return {success:false, message:'Tidak dapat terhubung ke server. Periksa koneksi dan coba lagi.'};
  }

  let data;
  try { data = await res.json(); }
  catch { data = {success:false, message:'Respons server tidak valid.'}; }

  if (res.status === 401) {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    window.location.replace('/index.html');
    return {success:false, message:'Sesi login sudah berakhir.'};
  }
  if (!res.ok) return {...data, success:false, status:res.status};
  return data;
};

window.formatCurrency = function(amount) {
  const value = Number(amount);
  return 'Rp' + (Number.isFinite(value) ? Math.trunc(value) : 0).toLocaleString('id-ID');
};
window.formatDate = function(dateString) {
  const date = new Date(dateString);
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleDateString('id-ID', {day:'numeric', month:'short', year:'numeric'});
};
window.toast = function(message, type='success') {
  let root = document.getElementById('toast-root');
  if (!root) { root = document.createElement('div'); root.id='toast-root'; root.className='fixed top-4 right-4 z-[100] space-y-2'; document.body.appendChild(root); }
  const item = document.createElement('div');
  item.className = `px-4 py-3 rounded-xl shadow-lg text-sm ${type === 'error' ? 'bg-red-600 text-white' : 'bg-gray-900 text-white'}`;
  item.textContent = message;
  root.appendChild(item);
  setTimeout(() => item.remove(), 3500);
};
window.confirmAction = function(message) { return window.confirm(message); };
window.logout = async function() {
  try { await window.api('/api/logout', {method:'POST'}); } catch (_) {}
  localStorage.removeItem('token'); localStorage.removeItem('user'); window.location.replace('/index.html');
};
window.hideLoading = function() { const el=document.getElementById('loading'); if(el) el.classList.add('hidden'); };
window.showLoading = function() { const el=document.getElementById('loading'); if(el) el.classList.remove('hidden'); };

/* Responsive mobile navigation */
window.initMobileMenu = function() {
  const button = document.getElementById('mobile-menu-button');
  const menu = document.getElementById('mobile-menu');
  if (!button || !menu || button.dataset.mobileReady === '1') return;
  button.dataset.mobileReady = '1';

  const setOpen = (open) => {
    menu.classList.toggle('hidden', !open);
    button.setAttribute('aria-expanded', open ? 'true' : 'false');
    document.body.classList.toggle('mobile-menu-open', open);
  };

  button.setAttribute('aria-expanded', 'false');
  button.setAttribute('aria-label', 'Buka menu navigasi');
  button.addEventListener('click', (event) => {
    event.stopPropagation();
    setOpen(menu.classList.contains('hidden'));
  });

  menu.querySelectorAll('a, button').forEach(el => {
    el.addEventListener('click', () => setOpen(false));
  });

  document.addEventListener('click', (event) => {
    if (!menu.classList.contains('hidden') && !menu.contains(event.target) && !button.contains(event.target)) {
      setOpen(false);
    }
  });

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') setOpen(false);
  });
};

// OCR Receipt Scanner
// Membaca struk dan mengekstrak data transaksi.
// Nominal diprioritaskan dari TOTAL/GRAND TOTAL/NOMINAL dan label total yang
// terpisah dari angka. Parser juga punya fallback untuk format OCR yang umum.
window.extractDataFromReceipt = function (file) {
  return new Promise((resolve, reject) => {
    if (!file || !file.type.startsWith('image/')) {
      reject(new Error('File struk harus berupa gambar.'));
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      reject(new Error('Ukuran foto struk maksimal 5MB.'));
      return;
    }

    const reader = new FileReader();
    reader.onload = function (event) {
      const imgElement = document.createElement('img');
      imgElement.onload = async function () {
        try {
          if (typeof Tesseract === 'undefined') throw new Error('Tesseract.js gagal dimuat.');

          // Batasi ukuran agar OCR browser stabil, tetapi tetap mempertahankan detail angka.
          const maxWidth = 1800;
          let width = imgElement.naturalWidth || imgElement.width;
          let height = imgElement.naturalHeight || imgElement.height;
          if (width > maxWidth) {
            const ratio = maxWidth / width;
            width = Math.round(width * ratio);
            height = Math.round(height * ratio);
          }

          const canvas = document.createElement('canvas');
          canvas.width = width;
          canvas.height = height;
          const ctx = canvas.getContext('2d', { willReadFrequently: true });
          ctx.drawImage(imgElement, 0, 0, width, height);

          // Grayscale + contrast ringan membantu angka nominal pada screenshot struk.
          const imageData = ctx.getImageData(0, 0, width, height);
          const data = imageData.data;
          for (let i = 0; i < data.length; i += 4) {
            const gray = Math.round(0.299 * data[i] + 0.587 * data[i + 1] + 0.114 * data[i + 2]);
            const contrast = Math.max(0, Math.min(255, Math.round((gray - 128) * 1.18 + 128)));
            data[i] = data[i + 1] = data[i + 2] = contrast;
          }
          ctx.putImageData(imageData, 0, 0);

          const result = await Tesseract.recognize(canvas, 'ind+eng', {
            logger: info => {
              if (info.status) console.log(`[OCR] ${info.status}`, info.progress ?? '');
            }
          });

          const text = result.data?.text || '';
          console.log('[OCR] Hasil teks mentah:', text);
          const parsed = parseReceiptText(text);
          console.log('[OCR] Data final:', parsed);

          if (!parsed.amount || parsed.amount <= 0) {
            throw new Error('Nominal tidak ditemukan dari foto struk.');
          }
          resolve(parsed);
        } catch (err) {
          console.error('[OCR] Error:', err);
          reject(err);
        }
      };
      imgElement.onerror = () => reject(new Error('Gambar struk tidak dapat dibaca.'));
      imgElement.src = event.target.result;
    };
    reader.onerror = () => reject(new Error('File struk tidak dapat dibaca.'));
    reader.readAsDataURL(file);
  });
};

function normalizeReceiptLine(line) {
  return line
    .replace(/[|]/g, 'I')
    .replace(/\bR[Pp][Oo0]?\s*([0-9])/g, 'Rp $1')
    .replace(/\bI[Dd][Rr][Oo0]?\s*([0-9])/g, 'IDR $1')
    .replace(/\b[Rr][Pp][Oo]\b/g, 'Rp')
    .replace(/\s+/g, ' ')
    .trim();
}

function parseMoney(raw) {
  if (!raw) return null;

  let value = String(raw)
    .replace(/[OoQqDd]/g, '0')
    .replace(/\s+/g, '')
    .replace(/[^0-9.,]/g, '');

  if (!value) return null;

  const digitsOnly = value.replace(/[.,]/g, '');
  if (!digitsOnly) return null;

  // Rupiah pada struk biasanya integer dengan pemisah ribuan.
  // 250.000 / 250,000 / 250000 => 250000.
  const dotCount = (value.match(/\./g) || []).length;
  const commaCount = (value.match(/,/g) || []).length;
  if (dotCount + commaCount > 0) {
    value = digitsOnly;
  }

  const amount = parseInt(value, 10);
  if (!Number.isFinite(amount)) return null;
  return amount;
}

function extractMoneyFromLine(line) {
  // Jangan membaca nomor referensi/rekening sebagai nominal bila sangat panjang.
  const cleaned = line.replace(/\b(?:rp|idr)\b/gi, ' ');
  const matches = cleaned.match(/\d[\d.,\s]*/g) || [];

  return matches
    .map(raw => raw.trim())
    .map(raw => ({ raw, value: parseMoney(raw) }))
    .filter(item => item.value !== null && item.value >= 100)
    .filter(item => {
      const digits = item.raw.replace(/[.,\s]/g, '');
      return digits.length <= 9;
    })
    .map(item => item.value);
}

function isDateLine(line) {
  return /\b\d{1,4}[\/\-]\d{1,2}[\/\-]\d{2,4}\b/.test(line) ||
         /\b\d{1,2}\s+(?:januari|februari|maret|april|mei|juni|juli|agustus|september|oktober|november|desember)\s+\d{4}\b/i.test(line);
}

function findReceiptDate(lines) {
  const months = {
    januari:'01', februari:'02', maret:'03', april:'04', mei:'05', juni:'06',
    juli:'07', agustus:'08', september:'09', oktober:'10', november:'11', desember:'12'
  };

  for (const line of lines) {
    let match = line.match(/\b(\d{1,2})[\/\-](\d{1,2})[\/\-](\d{4})\b/);
    if (match) {
      const day = match[1].padStart(2, '0');
      const month = match[2].padStart(2, '0');
      const year = match[3];
      const d = new Date(`${year}-${month}-${day}T00:00:00`);
      if (d.getFullYear() === Number(year) && d.getMonth() + 1 === Number(month) && d.getDate() === Number(day)) {
        return `${year}-${month}-${day}`;
      }
    }

    match = line.match(/\b(\d{1,2})\s+(januari|februari|maret|april|mei|juni|juli|agustus|september|oktober|november|desember)\s+(\d{4})\b/i);
    if (match) return `${match[3]}-${months[match[2].toLowerCase()]}-${match[1].padStart(2, '0')}`;
  }
  return null;
}

function findReceiptAmount(lines) {
  // Label yang paling kuat. Urutan penting: total transaksi/total bayar
  // didahulukan daripada subtotal, biaya, atau nominal item.
  const strongPatterns = [
    /\bgrand\s*total\b/i,
    /\btotal\s*transaksi\b/i,
    /\btotal\s*(?:bayar|pembayaran|tagihan|akhir)\b/i,
    /\btotal\b/i,
    /\bjumlah\s*(?:bayar|pembayaran|tagihan)?\b/i,
    /\bnet\s*total\b/i,
    /\bamount\s*(?:due|paid)?\b/i,
    /\bpayment\s*total\b/i,
    /\bnominal\b/i
  ];

  const candidates = [];

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (isDateLine(line)) continue;

    const matchedIndex = strongPatterns.findIndex(p => p.test(line));
    if (matchedIndex === -1) continue;

    // Angka pada baris label itu sendiri.
    for (const value of extractMoneyFromLine(line)) {
      candidates.push({ value, score: 5000 - matchedIndex * 100 - i });
    }

    // Banyak struk/screenshot: label dan nominal berada pada baris berikutnya.
    for (let j = 1; j <= 3 && i + j < lines.length; j++) {
      const nextLine = lines[i + j];
      if (isDateLine(nextLine)) continue;
      for (const value of extractMoneyFromLine(nextLine)) {
        candidates.push({ value, score: 4800 - matchedIndex * 100 - i - j * 10 });
      }
    }
  }

  if (candidates.length) {
    candidates.sort((a, b) => b.score - a.score || b.value - a.value);
    console.log('[OCR] Kandidat nominal berlabel:', candidates);
    return candidates[0].value;
  }

  // Fallback: kandidat dengan Rp/IDR di bagian bawah struk lebih dipercaya.
  const fallback = [];
  for (let i = 0; i < lines.length; i++) {
    if (isDateLine(lines[i])) continue;
    const values = extractMoneyFromLine(lines[i]);
    for (const value of values) {
      let score = i * 10 + Math.min(value / 10000, 100);
      if (/\b(?:rp|idr)\b/i.test(lines[i])) score += 500;
      fallback.push({ value, score, index: i });
    }
  }

  if (!fallback.length) return 0;
  fallback.sort((a, b) => b.score - a.score || b.value - a.value || b.index - a.index);
  console.log('[OCR] Kandidat nominal fallback:', fallback);
  return fallback[0].value;
}

function cleanReceiptTitle(value) {
  return String(value || '')
    .replace(/^[\s:：-]+|[\s:：-]+$/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

function isUsefulTitle(line) {
  const blocked = /^(?:total|grand total|jumlah|subtotal|sub total|cash|tunai|kembali|change|tanggal|date|tgl|jam|time|telp|phone|alamat|address|nota|invoice|struk|transaksi|transaksi berhasil|no\.?\s*(?:ref|referensi|rekening)|ref(?:erence)?|nominal|biaya admin|admin|informasi|sumber dana|tujuan|bank|rekening|account|atas nama|nama|jenis transaksi|catatan|status)\b/i;
  if (blocked.test(line)) return false;
  if (isDateLine(line)) return false;
  if (extractMoneyFromLine(line).length) return false;
  if (/^[A-Z0-9 .\-_/]{1,12}$/.test(line) && !/[a-zA-Z]{3,}/.test(line)) return false;
  if (/^\d[\d\s*+()-]{5,}$/.test(line)) return false;
  return /[a-zA-ZÀ-ÿ]/.test(line) && line.length >= 2 && line.length <= 60;
}

function findReceiptTitle(lines) {
  // 1) Struk transfer bank: gunakan nama penerima setelah label Tujuan/Penerima.
  const recipientLabels = /^(?:tujuan|penerima|beneficiary|recipient|dikirim ke|transfer ke)\s*[:：-]?\s*(.*)$/i;
  for (let i = 0; i < lines.length; i++) {
    const match = lines[i].match(recipientLabels);
    if (match && match[1] && isUsefulTitle(match[1])) {
      return `Transfer ke ${cleanReceiptTitle(match[1])}`;
    }
    if (match && i + 1 < lines.length && isUsefulTitle(lines[i + 1])) {
      return `Transfer ke ${cleanReceiptTitle(lines[i + 1])}`;
    }
  }

  // 2) Merchant/toko yang muncul setelah label nama merchant/toko.
  const merchantLabels = /^(?:merchant|nama merchant|nama toko|toko|store|outlet|cabang|seller|penjual)\s*[:：-]?\s*(.*)$/i;
  for (let i = 0; i < lines.length; i++) {
    const match = lines[i].match(merchantLabels);
    if (match && match[1] && isUsefulTitle(match[1])) return cleanReceiptTitle(match[1]);
    if (match && i + 1 < lines.length && isUsefulTitle(lines[i + 1])) return cleanReceiptTitle(lines[i + 1]);
  }

  // 3) Cari nama bank/provider sebagai konteks transfer. Jangan memilih nama
  // pemilik rekening sumber sebagai judul transaksi.
  const hasBankTransfer = /\b(?:transfer|bank|BRI|BNI|BCA|Mandiri|BSI|BTN|BRImo|Livin|myBCA|mobile banking|internet banking)\b/i.test(lines.join(' '));
  if (hasBankTransfer) {
    for (const line of lines) {
      if (!isUsefulTitle(line)) continue;
      if (/^(?:bank|BRI|BNI|BCA|Mandiri|BSI|BTN|BRImo|Livin|myBCA|mobile banking|internet banking)\b/i.test(line)) continue;
      // Nama orang biasanya 2-4 kata; prioritaskan kandidat yang bukan label UI.
      const words = line.split(/\s+/).filter(Boolean);
      if (words.length >= 2 && words.length <= 5 && /[A-Za-zÀ-ÿ]/.test(line)) {
        return `Transfer ke ${cleanReceiptTitle(line)}`;
      }
    }
  }

  // 4) Fallback: kandidat bagian atas, tetapi hindari label/form yang umum.
  const candidates = lines.slice(0, Math.min(lines.length, 15)).filter(isUsefulTitle);
  return cleanReceiptTitle(candidates[0] || 'Tanpa judul');
}

function detectReceiptCategory(text) {
  const lower = text.toLowerCase();

  if (/makan|resto|restaurant|kafe|cafe|warung|food|bakery|bakso|ayam|mie|kopi|coffee|pizza|burger|kfc|mcdonald|mcd|starbucks/.test(lower)) return 'Makanan';
  if (/transport|bensin|pertamina|shell|taksi|taxi|grab|gojek|uber|bus|kereta|toll|tol|parkir/.test(lower)) return 'Transportasi';
  if (/belanja|market|hypermart|alfamart|indomaret|superindo|carrefour|supermarket|minimarket|department store/.test(lower)) return 'Belanja';
  if (/hiburan|bioskop|cinema|xxi|cgv|game|playstation|steam/.test(lower)) return 'Hiburan';
  if (/listrik|pln|air|pdam|telkom|internet|wifi|tagihan|bill/.test(lower)) return 'Tagihan';
  if (/sekolah|kuliah|kampus|buku|education|pendidikan|course|kursus/.test(lower)) return 'Pendidikan';
  return 'Lainnya';
}

function parseReceiptText(text) {
  const lines = text
    .split(/\r?\n/)
    .map(normalizeReceiptLine)
    .filter(Boolean);

  const normalizedText = lines.join(' ');
  const amount = findReceiptAmount(lines);
  const date = findReceiptDate(lines);
  const title = findReceiptTitle(lines);
  const category = detectReceiptCategory(normalizedText);

  console.log('[OCR] Data hasil parsing:', { title, amount, category, date, lines });

  return {
    title,
    amount,
    category,
    date: date || new Date().toISOString().split('T')[0]
  };
}
