package domain

// Programa es una emisión de la guía EPG (XMLTV) de una fuente. ChannelID es
// el channel_id XMLTV — el mismo valor que channels.tvg_id para un canal de
// esa fuente —, NUNCA el ChannelID de la app: una fila de epg_programmes no
// sabe (ni le hace falta saber) a qué canal de la app termina uniéndose; eso
// lo decide el join (provider_id, tvg_id) en tiempo de lectura. Ver
// ports.EPGRepository.
type Programa struct {
	ChannelID   string
	InicioUTC   int64
	FinUTC      int64
	Titulo      string
	Subtitulo   string
	Descripcion string
}

// AhoraDespues es lo que se emite ahora y lo próximo en un canal de la app.
// Punteros: nil = sin dato en ese hueco (nunca "programa vacío" con campos a
// blanco). Ver ports.EPGRepository.AhoraDespuesPorCanales para cuándo un
// canal ni siquiera aparece en el mapa de resultados.
type AhoraDespues struct {
	Ahora     *Programa
	Siguiente *Programa
}
