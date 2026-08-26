<script lang="ts">
  import { t } from '../i18n'

  // Tarea 5 (P0.6): indicador honesto de señal en la cabecera. estado llega
  // YA resuelto desde App (deriva de /health vía consultarSalud() y de
  // clasificarError() cuando /health falla) — este componente no vuelve a
  // decidir nada, solo pinta el mapeo estado→{clase,claveTexto}. Nunca
  // decorativo: si el gateway se cae, este punto lo dice, no queda en verde.
  let { estado }: { estado: 'vivo' | 'sincronizando' | 'sin-gateway' } = $props()

  // Mapa único estado→{clase,claveTexto}: la clase gobierna el color del
  // punto (vía CSS, tokens de colors.css) y claveTexto el texto adyacente —
  // ambos MISMOS por estado, así el color nunca es la única señal
  // (accesibilidad: brief Tarea 5, "comunica el estado por TEXTO, no solo
  // color"). Ámbar rationed: solo 'sincronizando' lo usa aquí.
  const mapa = {
    vivo: { clase: 'vivo', claveTexto: 'indicador.vivo' },
    sincronizando: { clase: 'sincronizando', claveTexto: 'indicador.sincronizando' },
    'sin-gateway': { clase: 'sin-gateway', claveTexto: 'indicador.sinGateway' },
  } as const

  const info = $derived(mapa[estado])
</script>

<span class="indicador" data-estado={estado}>
  <i class="punto {info.clase}" aria-hidden="true"></i>
  <span class="texto">{t(info.claveTexto)}</span>
</span>

<style>
  .indicador {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-muted);
  }
  .punto {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  /* vivo: pine, NUNCA ámbar — el ámbar queda rationed a 'sincronizando'. */
  .punto.vivo { background: var(--signal-ok); }
  .punto.sincronizando { background: var(--amber-500); }
  .punto.sin-gateway { background: var(--text-faint); }
</style>
