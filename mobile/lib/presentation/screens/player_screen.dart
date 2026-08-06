import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';
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
  Timer? _timeoutTimer;

  @override
  void initState() {
    super.initState();
    _player = Player();
    _controller = VideoController(_player);
    _loadAndPlay();
  }

  Future<void> _loadAndPlay() async {
    _timeoutTimer?.cancel();
    for (final s in _subs) {
      s.cancel();
    }
    _subs.clear();

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final repo = ref.read(channelRepositoryProvider);
      final streamUrl = await repo.getStreamUrl(widget.channelId);

      if (_player.platform is NativePlayer) {
        final mpv = _player.platform as NativePlayer;
        await mpv.setProperty('network-timeout', '10');
        await mpv.setProperty('demuxer-max-bytes', '4MiB');
      }

      await _player.open(Media(streamUrl));

      _subs.add(_player.stream.playing.listen((playing) {
        if (playing && _isLoading && mounted) {
          _timeoutTimer?.cancel();
          setState(() => _isLoading = false);
        }
      }));

      _subs.add(_player.stream.error.listen((err) {
        if (err.isNotEmpty && mounted) {
          _timeoutTimer?.cancel();
          setState(() {
            _isLoading = false;
            _error = err;
          });
        }
      }));

      _timeoutTimer = Timer(_kPlayTimeout, () {
        if (_isLoading && mounted) {
          setState(() {
            _isLoading = false;
            _error =
                'El canal no respondió en ${_kPlayTimeout.inSeconds}s.\n'
                'Puede estar offline o la URL expiró.';
          });
        }
      });
    } catch (e) {
      _timeoutTimer?.cancel();
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
    _timeoutTimer?.cancel();
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
