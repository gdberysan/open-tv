import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';
import '../player/playback_guard.dart';
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
    _guard?.dispose();
    for (final s in _subs) {
      s.cancel();
    }
    _subs.clear();

    setState(() {
      _isLoading = true;
      _error = null;
    });

    final guard = PlaybackGuard(onFatal: _onFatal, loadTimeout: _kPlayTimeout);
    _guard = guard;

    try {
      final repo = ref.read(channelRepositoryProvider);
      final streamUrl = await repo.getStreamUrl(widget.channelId);

      if (_player.platform is NativePlayer) {
        final mpv = _player.platform as NativePlayer;
        await mpv.setProperty('network-timeout', '10');
        await mpv.setProperty('demuxer-max-bytes', '4MiB');
      }

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

      // Armar ANTES de open(): si open se cuelga, el timeout salta igual.
      guard.armLoadTimeout();
      await _player.open(Media(streamUrl));
    } catch (e) {
      guard.dispose();
      if (mounted) {
        setState(() {
          _isLoading = false;
          _error = e.toString();
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
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.signal_wifi_off, color: Colors.red, size: 56),
            const SizedBox(height: 16),
            Text(
              'Canal no disponible',
              style: Theme.of(context)
                  .textTheme
                  .titleMedium
                  ?.copyWith(color: Colors.white),
            ),
            const SizedBox(height: 8),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 32),
              child: Text(
                _error!,
                textAlign: TextAlign.center,
                style: const TextStyle(color: Colors.grey, fontSize: 12),
              ),
            ),
            const SizedBox(height: 24),
            ElevatedButton.icon(
              onPressed: _loadAndPlay,
              icon: const Icon(Icons.play_arrow),
              label: const Text('Reintentar'),
            ),
          ],
        ),
      );
    }

    return Stack(
      children: [
        Video(controller: _controller),
        if (_isLoading)
          Container(
            color: Colors.black,
            child: Center(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  const CircularProgressIndicator(),
                  const SizedBox(height: 16),
                  Text(
                    'Cargando ${widget.channelName}…',
                    style: const TextStyle(color: Colors.white70),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Timeout en ${_kPlayTimeout.inSeconds}s',
                    style:
                        const TextStyle(color: Colors.grey, fontSize: 10),
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
