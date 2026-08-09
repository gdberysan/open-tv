import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';
import '../../data/api_error.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../player/playback_guard.dart';
import '../widgets/console_line.dart';
import '../widgets/korven_emblem.dart';
import '../widgets/state_views.dart';
import '../providers/channel_provider.dart';

const _kPlayTimeout = Duration(seconds: 15);

class PlayerScreen extends ConsumerStatefulWidget {
  final String channelId;
  final String channelName;
  final String countryCode;

  const PlayerScreen({
    required this.channelId,
    required this.channelName,
    this.countryCode = '',
    super.key,
  });

  @override
  ConsumerState<PlayerScreen> createState() => _PlayerScreenState();
}

class _PlayerScreenState extends ConsumerState<PlayerScreen> {
  late final Player _player;
  late final VideoController _controller;

  bool _isLoading = true;
  String? _error;

  final List<StreamSubscription<dynamic>> _subs = [];
  PlaybackGuard? _guard;

  /// Generación de carga. Cada _loadAndPlay la incrementa; una invocación
  /// anterior que siga en vuelo se descarta al volver de su await en vez de
  /// pisar el estado de la más reciente. Sin esto, un doble toque en
  /// "Reintentar" hacía que la primera carga añadiese sus suscripciones a un
  /// _subs ya vaciado por la segunda y abriese su URL obsoleta.
  int _generacion = 0;

  @override
  void initState() {
    super.initState();
    _player = Player();
    _controller = VideoController(_player);
    _loadAndPlay();
  }

  /// Fatal real decidido por el PlaybackGuard: parar el player (si no, el
  /// audio sigue sonando detrás de la pantalla de error) y mostrar el fallo.
  void _onFatal(String message) {
    _player.stop();
    if (mounted) {
      setState(() {
        _isLoading = false;
        _error = message;
      });
    }
  }

  Future<void> _loadAndPlay() async {
    final generacion = ++_generacion;
    final esRecarga = generacion > 1;

    _guard?.dispose();
    for (final s in _subs) {
      s.cancel();
    }
    _subs.clear();
    if (esRecarga) {
      // Si no, el audio del intento anterior sigue sonando bajo el overlay.
      await _player.stop();
    }

    setState(() {
      _isLoading = true;
      _error = null;
    });

    final guard = PlaybackGuard(onFatal: _onFatal, loadTimeout: _kPlayTimeout);
    _guard = guard;

    // Armar ANTES de resolver la URL, no solo antes de open(): el fetch al
    // gateway puede colgarse igual que open(), y el overlay promete un timeout
    // de _kPlayTimeout para todo el proceso. Con el guard armado después del
    // await, un gateway que acepta el TCP y no responde dejaba la pantalla en
    // "Cargando… / Timeout en 15s" para siempre, con el botón de reintentar
    // oculto porque depende de !_isLoading.
    guard.armLoadTimeout();

    try {
      final repo = ref.read(channelRepositoryProvider);
      final streamUrl = await repo.getStreamUrl(widget.channelId);
      if (generacion != _generacion) return;

      if (_player.platform is NativePlayer) {
        final mpv = _player.platform as NativePlayer;
        await mpv.setProperty('network-timeout', '10');
        await mpv.setProperty('demuxer-max-bytes', '4MiB');
      }
      if (generacion != _generacion) return;

      _subs.add(_player.stream.playing.listen((playing) {
        guard.onPlaying(playing);
        if (playing && _isLoading && mounted) {
          setState(() => _isLoading = false);
        }
      }));
      _subs.add(_player.stream.position.listen(guard.onPosition));
      // Los errores de mpv pasan por el guard: los transitorios de HLS en
      // vivo (EOF de segmento, reconexiones) NO matan la reproducción.
      _subs.add(_player.stream.error.listen(guard.onError));

      await _player.open(Media(streamUrl));
    } catch (e) {
      if (generacion != _generacion) return;
      guard.dispose();
      if (mounted) {
        setState(() {
          _isLoading = false;
          _error = ApiError.desde(e).mensaje;
        });
      }
    }
  }

  @override
  void dispose() {
    _guard?.dispose();
    for (final s in _subs) {
      s.cancel();
    }
    _player.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: Row(
          children: [
            if (widget.countryCode.isNotEmpty) ...[
              Text(_flag(widget.countryCode),
                  style: const TextStyle(fontSize: 20)),
              const SizedBox(width: 8),
            ],
            Expanded(child: Text(widget.channelName)),
          ],
        ),
        actions: [
          if (!_isLoading)
            IconButton(
              icon: const Icon(Icons.refresh, color: Colors.white),
              tooltip: 'Reintentar',
              onPressed: _loadAndPlay,
            ),
        ],
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_error != null) {
      return KorvenStateView(
        eyebrow: '// canal no disponible',
        message: _error!,
        action: ElevatedButton.icon(
          onPressed: _loadAndPlay,
          icon: const Icon(Icons.play_arrow, size: 18),
          label: const Text('Reintentar'),
        ),
      );
    }

    return Stack(
      children: [
        Video(controller: _controller),
        if (_isLoading)
          Container(
            color: KorvenColors.surfaceBase,
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Opacity(opacity: 0.5, child: KorvenEmblem(size: 64)),
                  const SizedBox(height: KorvenSpacing.s5),
                  // Nombra lo que está pasando, en vez de un spinner que solo
                  // dice "algo ocurre". El watchdog de 15s sigue igual.
                  ConsoleLine(
                    text: '\$ korven tune --channel "${widget.channelName}"',
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }

  String _flag(String cc) {
    if (cc.length != 2) return '';
    final a = cc.toUpperCase().codeUnitAt(0) - 0x41 + 0x1F1E6;
    final b = cc.toUpperCase().codeUnitAt(1) - 0x41 + 0x1F1E6;
    return String.fromCharCode(a) + String.fromCharCode(b);
  }
}
